package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/mgrachev/errorformat"
	"github.com/mgrachev/errorformat/fmts"
	"github.com/mgrachev/errorformat/writer"
)

const usageMessage = "" +
	`Usage: errorformat [flags] [errorformat ...]

errorformat reads compiler/linter/static analyzer result from STDIN, formats
them by given 'errorformat' (90% compatible with Vim's errorformat. :h
errorformat), and outputs formated result to STDOUT.

Example:
	$ echo '/path/to/file:14:28: error message\nfile2:3:4: msg' | errorformat "%f:%l:%c: %m"
	/path/to/file|14 col 28| error message
	file2|3 col 4| msg

	$ golint ./... | errorformat -name=golint

The -f flag specifies an alternate format for the entry, using the
syntax of package template.  The default output is equivalent to -f
'{{.String}}'. The struct being passed to the template is:

	type Entry struct {
		// name of a file
		Filename string
		// line number
		Lnum int
		// column number (first column is 1)
		Col int
		// true: "col" is visual column
		// false: "col" is byte index
		Vcol bool
		// error number
		Nr int
		// search pattern used to locate the error
		Pattern string
		// description of the error
		Text string
		// type of the error, 'E', '1', etc.
		Type rune
		// true: recognized error message
		Valid bool

		// Original error lines (often one line. more than one line for multi-line
		// errorformat. :h errorformat-multi-line)
		Lines []string
	}
`

func usage() {
	fmt.Fprintln(os.Stderr, usageMessage)
	fmt.Fprintln(os.Stderr, "Flags:")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	var (
		entryFmt  = flag.String("f", "{{.String}}", "format template for -w=template")
		writerFmt = flag.String("w", "template", "writer format (template|checkstyle)")
		name      = flag.String("name", "", "defined errorformat name")
		list      = flag.Bool("list", false, "list defined errorformats")
	)
	flag.Usage = usage
	flag.Parse()
	errorformats := flag.Args()
	if err := run(os.Stdin, os.Stdout, errorformats, *writerFmt, *entryFmt, *name, *list); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(r io.Reader, w io.Writer, efms []string, writerFmt, entryFmt, name string, list bool) error {
	if list {
		fs := fmts.DefinedFmts()
		out := make([]string, 0, len(fs))
		for _, f := range fs {
			out = append(out, fmt.Sprintf("%s\t\t%s - %s", f.Name, f.Description, f.URL))
		}
		sort.Strings(out)
		fmt.Fprintln(w, strings.Join(out, "\n"))
		return nil
	}

	if name != "" {
		f, ok := fmts.DefinedFmts()[name]
		if !ok {
			return fmt.Errorf("%q is not defined", name)
		}
		efms = f.Errorformat
	}

	var ewriter writer.Writer

	switch writerFmt {
	case "template", "":
		fm := template.FuncMap{
			"join": strings.Join,
		}
		tmpl, err := template.New("main").Funcs(fm).Parse(entryFmt)
		if err != nil {
			return err
		}
		ewriter = writer.NewTemplate(tmpl, w)
	case "checkstyle":
		ewriter = writer.NewCheckStyle(w)
	default:
		return fmt.Errorf("unknown writer: -w=%v", writerFmt)
	}
	if ewriter, ok := ewriter.(writer.BufWriter); ok {
		defer func() {
			if err := ewriter.Flush(); err != nil {
				log.Println(err)
			}
		}()
	}

	efm, err := errorformat.NewErrorformat(efms)
	if err != nil {
		return err
	}
	s := efm.NewScanner(r)
	for s.Scan() {
		if err := ewriter.Write(s.Entry()); err != nil {
			return err
		}
	}
	return nil
}

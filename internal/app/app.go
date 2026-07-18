package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/juanandrade/prompt-cli/internal/editor"
	"github.com/juanandrade/prompt-cli/internal/lsp"
)

var Version = "dev"

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		usage(stdout)
		return 0
	}
	if len(args) > 0 && (args[0] == "version" || args[0] == "--version") {
		fmt.Fprintln(stdout, Version)
		return 0
	}
	if len(args) > 0 && args[0] == "lsp" {
		return runLSP(args[1:], stderr)
	}
	if len(args) > 0 && args[0] == "edit" {
		args = args[1:]
	}
	fs := flag.NewFlagSet("prompt-cli", flag.ContinueOnError)
	fs.SetOutput(stderr)
	workspace := fs.String("workspace", "", "workspace root")
	line := fs.Int("line", 1, "initial one-based line")
	column := fs.Int("column", 1, "initial one-based column")
	gotoMode := fs.Bool("goto", false, "parse FILE:LINE[:COLUMN]")
	_ = fs.Bool("wait", true, "wait until the editor closes (accepted for editor CLI compatibility)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "prompt-cli: expected at most one file")
		return 2
	}
	anonymous := fs.NArg() == 0
	path := ""
	if !anonymous {
		path = fs.Arg(0)
	}
	if *gotoMode {
		if anonymous {
			fmt.Fprintln(stderr, "prompt-cli: --goto requires a file")
			return 2
		}
		path, *line, *column = parseGoto(path, *line, *column)
	}
	root := *workspace
	var err error
	if root == "" {
		root, err = os.Getwd()
	} else {
		root, err = filepath.Abs(root)
	}
	if err != nil {
		fmt.Fprintln(stderr, "prompt-cli:", err)
		return 1
	}
	var e *editor.Editor
	if anonymous {
		e, err = editor.NewUntitled(root, *line, *column)
	} else {
		var abs string
		abs, err = filepath.Abs(path)
		if err == nil {
			e, err = editor.New(abs, root, *line, *column)
		}
	}
	output := ""
	if err == nil {
		output, err = e.Run()
	}
	if errors.Is(err, editor.ErrCancelled) {
		return 130
	}
	if err != nil {
		fmt.Fprintln(stderr, "prompt-cli:", err)
		return 1
	}
	if anonymous {
		if writeErr := writeOutput(stdout, output); writeErr != nil {
			fmt.Fprintln(stderr, "prompt-cli: write output:", writeErr)
			return 1
		}
	}
	return 0
}

func writeOutput(w io.Writer, text string) error {
	n, err := io.WriteString(w, text)
	if err != nil {
		return err
	}
	if n != len(text) {
		return io.ErrShortWrite
	}
	return nil
}

func runLSP(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("prompt-cli lsp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	workspace := fs.String("workspace", "", "authoritative workspace root")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "prompt-cli lsp: unexpected arguments")
		return 2
	}
	explicit := *workspace != ""
	root := *workspace
	var err error
	if root == "" {
		root, err = os.Getwd()
	} else {
		root, err = filepath.Abs(root)
	}
	if err == nil {
		err = lsp.Run(root, explicit)
	}
	if err != nil {
		fmt.Fprintln(stderr, "prompt-cli lsp:", err)
		return 1
	}
	return 0
}

func parseGoto(value string, line, column int) (string, int, int) {
	parts := strings.Split(value, ":")
	if len(parts) < 2 {
		return value, line, column
	}
	last, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return value, line, column
	}
	if len(parts) >= 3 {
		if before, e := strconv.Atoi(parts[len(parts)-2]); e == nil {
			return strings.Join(parts[:len(parts)-2], ":"), before, last
		}
	}
	return strings.Join(parts[:len(parts)-1], ":"), last, column
}

func usage(w io.Writer) {
	fmt.Fprintln(w, `prompt-cli — Prompt Lang Markdown editor

Usage:
  prompt-cli [edit] [--workspace DIR] [--line N --column N] [FILE]
  prompt-cli --goto FILE:LINE[:COLUMN]
  prompt-cli lsp [--workspace DIR]
  prompt-cli version

Without FILE, Ctrl+S returns Markdown on stdout without writing to disk.
Inside the editor: Ctrl+S confirms, Ctrl+X/Ctrl+C asks before canceling, Ctrl+Space completes.`)
}

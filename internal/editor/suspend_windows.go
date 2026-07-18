//go:build windows

package editor

import (
	"golang.org/x/term"
	"io"
)

func suspendTerminal(_ int, _ *term.State, _ io.Writer) error { return nil }

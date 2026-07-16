//go:build windows

package editor

import "golang.org/x/term"

func suspendTerminal(_ int, _ *term.State) error { return nil }

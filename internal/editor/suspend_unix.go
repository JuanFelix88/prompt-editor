//go:build !windows

package editor

import (
	"io"
	"os"
	"syscall"

	"golang.org/x/term"
)

func suspendTerminal(fd int, old *term.State) error {
	_, _ = io.WriteString(os.Stdout, "\x1b[?2004l\x1b[?1049l\x1b[?25h\x1b[0m")
	if err := term.Restore(fd, old); err != nil {
		return err
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGTSTP); err != nil {
		return err
	}
	_, err := term.MakeRaw(fd)
	if err == nil {
		_, _ = io.WriteString(os.Stdout, "\x1b[?1049h\x1b[2J\x1b[H\x1b[?2004h\x1b[?25h")
	}
	return err
}

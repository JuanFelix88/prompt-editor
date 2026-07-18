package app

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestWriteOutputExactAndErrors(t *testing.T) {
	var out bytes.Buffer
	if err := writeOutput(&out, "# prompt\nbody"); err != nil || out.String() != "# prompt\nbody" {
		t.Fatalf("output=%q err=%v", out.String(), err)
	}
	if err := writeOutput(failingWriter{}, "x"); err == nil {
		t.Fatal("write error ignored")
	}
	if err := writeOutput(shortWriter{}, "x"); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write err=%v", err)
	}
}

func TestParseGoto(t *testing.T) {
	path, line, col := parseGoto("C:\\work\\prompt.md:12:4", 1, 1)
	if path != "C:\\work\\prompt.md" || line != 12 || col != 4 {
		t.Fatalf("got %q %d %d", path, line, col)
	}
	path, line, col = parseGoto("prompt.md:7", 1, 3)
	if path != "prompt.md" || line != 7 || col != 3 {
		t.Fatalf("got %q %d %d", path, line, col)
	}
}

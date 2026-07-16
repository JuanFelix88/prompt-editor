package app

import "testing"

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

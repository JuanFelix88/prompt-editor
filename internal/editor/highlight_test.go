package editor

import (
	"testing"

	"github.com/juanandrade/prompt-cli/internal/lang"
)

func TestPromptItemStylesUseTerminalThemeColors(t *testing.T) {
	if got := tokenStyle(lang.TokenPromptItem); got != "\x1b[31m" {
		t.Fatalf("hyphen item style=%q, want ANSI theme red", got)
	}
	if got := tokenStyle(lang.TokenPromptStarItem); got != "\x1b[34m" {
		t.Fatalf("star item style=%q, want ANSI theme blue", got)
	}
}

func TestTaskMarkerStylesUseTerminalThemeColors(t *testing.T) {
	if got := tokenStyle(lang.TokenTaskDone); got != "\x1b[90m" {
		t.Fatalf("done marker style=%q, want ANSI theme gray", got)
	}
	if got := tokenStyle(lang.TokenTaskMarker); got != "\x1b[33m" {
		t.Fatalf("custom marker style=%q, want ANSI theme yellow", got)
	}
}

// The structural pass colors task markers too, so a document without
// contextual tokens keeps the same color, and fenced lines stay untouched.
func TestStructuralTaskMarkerStyles(t *testing.T) {
	lines := [][]rune{[]rune("- [x] done"), []rune("* [*] custom"), []rune("- [ ] pending")}
	e := &Editor{buffer: &Buffer{Lines: lines}}
	for _, tc := range []struct {
		line int
		want string
	}{{0, "\x1b[90m"}, {1, "\x1b[33m"}} {
		styles := e.structuralStyles(tc.line, lines[tc.line], false, false, false)
		if styles[3] != tc.want {
			t.Errorf("line %q marker style=%q want %q", string(lines[tc.line]), styles[3], tc.want)
		}
		// The brackets keep the color of the line they belong to.
		if styles[2] == tc.want || styles[4] == tc.want {
			t.Errorf("line %q brackets were colored: %#v", string(lines[tc.line]), styles)
		}
	}
	if styles := e.structuralStyles(2, lines[2], false, false, false); styles[3] != "" {
		t.Errorf("unchecked marker style=%q want none", styles[3])
	}
	fenced := []rune("- [x] inside a fence")
	if styles := e.structuralStyles(0, fenced, true, false, false); styles[3] != "\x1b[38;5;109m" {
		t.Errorf("fenced marker style=%q want generic code style", styles[3])
	}
}

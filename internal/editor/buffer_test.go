package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/juanandrade/prompt-cli/internal/lang"
)

func TestBufferEditUnicodeAndSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prompt.md")
	b, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	b.InsertText("a👩‍💻界")
	b.Left()
	if b.Col != 4 {
		t.Fatalf("left over CJK col=%d", b.Col)
	}
	b.Left()
	if b.Col != 1 {
		t.Fatalf("left over emoji cluster col=%d", b.Col)
	}
	b.Newline()
	b.InsertText("next\x1b[31m")
	if got := b.Text(); strings.ContainsRune(got, '\x1b') {
		t.Fatalf("unsanitized text=%q", got)
	}
	if err := b.Save(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != b.Text() {
		t.Fatalf("disk=%q buffer=%q", data, b.Text())
	}
}
func TestAutoPairAndBackspace(t *testing.T) {
	e := &Editor{buffer: &Buffer{Lines: [][]rune{{}}}}
	e.handle(Key{Kind: KeyRune, Rune: '['})
	if e.buffer.Text() != "[]" || e.buffer.Col != 1 {
		t.Fatalf("pair text=%q col=%d", e.buffer.Text(), e.buffer.Col)
	}
	e.handle(Key{Kind: KeyBackspace})
	if e.buffer.Text() != "" || e.buffer.Col != 0 {
		t.Fatalf("pair delete text=%q col=%d", e.buffer.Text(), e.buffer.Col)
	}
}

func TestCompletionWindowFollowsSelection(t *testing.T) {
	if start := completionWindow(250, 500, 8); start > 250 || start+8 <= 250 {
		t.Fatalf("window start=%d", start)
	}
}

func TestApplyCompletionUTF16(t *testing.T) {
	b := &Buffer{Lines: [][]rune{[]rune("😀 @do")}, Col: 5}
	c := lang.Completion{TextEdit: lang.TextEdit{Range: lang.Range{Start: lang.Position{Line: 0, Character: 4}, End: lang.Position{Line: 0, Character: 6}}, NewText: "docs/"}}
	b.ApplyCompletion(c)
	if got := b.Text(); got != "😀 @docs/" {
		t.Fatalf("got %q", got)
	}
}

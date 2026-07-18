package editor

import (
	"fmt"
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

func TestDeleteWordBackward(t *testing.T) {
	b := &Buffer{Lines: [][]rune{[]rune("olá mundo")}, Col: 9}
	b.DeleteWordBackward()
	if b.Text() != "olá " || b.Col != 4 {
		t.Fatalf("text=%q col=%d", b.Text(), b.Col)
	}
	b.DeleteWordBackward()
	if b.Text() != "" || b.Col != 0 {
		t.Fatalf("second delete text=%q col=%d", b.Text(), b.Col)
	}
	b = &Buffer{Lines: [][]rune{[]rune("go ✈️")}, Col: len([]rune("go ✈️"))}
	b.DeleteWordBackward()
	if b.Text() != "go " {
		t.Fatalf("grapheme split: %q", b.Text())
	}
}

func TestInsertLineBelowPreservesFocusedLine(t *testing.T) {
	b := &Buffer{Lines: [][]rune{[]rune("  keep this"), []rune("next")}, Row: 0, Col: 6}
	b.InsertLineBelow()
	if got := b.Text(); got != "  keep this\n  \nnext" {
		t.Fatalf("text=%q", got)
	}
	if b.Row != 1 || b.Col != 2 {
		t.Fatalf("row=%d col=%d", b.Row, b.Col)
	}
}

func TestCancelRequiresConfirmation(t *testing.T) {
	e := &Editor{buffer: &Buffer{Lines: [][]rune{{}}}}
	if _, _, exit := e.handle(Key{Kind: KeyCancel}); exit != 0 || !e.confirmCancel {
		t.Fatalf("first cancel exited=%d confirm=%v", exit, e.confirmCancel)
	}
	e.handle(Key{Kind: KeyEnter})
	if e.confirmCancel {
		t.Fatal("Enter did not choose default No")
	}
	e.handle(Key{Kind: KeyCancel})
	e.handle(Key{Kind: KeyEscape})
	if e.confirmCancel {
		t.Fatal("Escape did not close confirmation")
	}
	e.handle(Key{Kind: KeyCancel})
	e.handle(Key{Kind: KeyRune, Rune: 'n'})
	if e.confirmCancel {
		t.Fatal("n did not close confirmation")
	}
	e.handle(Key{Kind: KeyCancel})
	if _, _, exit := e.handle(Key{Kind: KeyRune, Rune: 'y'}); exit != -1 {
		t.Fatalf("y exit=%d", exit)
	}
}

func TestCancelModalOwnsLiquidCursorFocus(t *testing.T) {
	e := &Editor{buffer: &Buffer{Lines: [][]rune{[]rune("text")}}, rows: 24, cols: 80, confirmCancel: true}
	prompt := clipPlain(" Descartar e sair? [y/N]", e.cols-1)
	want := fmt.Sprintf("\x1b[%d;%dH\x1b[?25h", e.rows, min(visualWidth([]rune(prompt)), e.cols-1)+1)
	if frame := e.frame(); !strings.HasSuffix(frame, want) {
		t.Fatalf("frame does not end at modal focus: want suffix %q", want)
	}
}

func TestCompletionWindowFollowsSelection(t *testing.T) {
	if start := completionWindow(250, 500, 8); start > 250 || start+8 <= 250 {
		t.Fatalf("window start=%d", start)
	}
}

func TestUntitledConfirmReturnsTextWithoutDiskWrite(t *testing.T) {
	root := t.TempDir()
	e, err := NewUntitled(root, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	e.buffer.InsertText("# Output\nbody")
	output, done := e.confirm()
	if !done || output != "# Output\nbody" {
		t.Fatalf("done=%v output=%q", done, output)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("unexpected disk files: %v", entries)
	}
}

func TestUntitledEditorHasNoDiskPath(t *testing.T) {
	e, err := NewUntitled(t.TempDir(), 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !e.anonymous || e.buffer.Path != "" {
		t.Fatalf("anonymous=%v path=%q", e.anonymous, e.buffer.Path)
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

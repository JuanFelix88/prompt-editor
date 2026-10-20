package editor

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/juanandrade/prompt-cli/internal/lang"
)

func writeExternal(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}

func externalEditor(t *testing.T, text string) *Editor {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "prompt.md")
	writeExternal(t, path, text)
	e, err := New(path, root, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	e.cols, e.rows = 120, 24
	return e
}

func TestExternalReloadRefreshesEditor(t *testing.T) {
	e := externalEditor(t, "```\nold\n```\nlast line")
	b := e.buffer
	b.SetPosition(4, 10)
	e.top, e.hscroll, e.keepOpen = 3, 5, true
	e.completions = []lang.Completion{{Label: "stale"}}
	e.structuralStates()
	version := b.Version
	writeExternal(t, b.Path, "# New\ne\u0301")
	if !e.checkExternal() {
		t.Fatal("external change did not trigger a redraw")
	}
	if e.buffer != b || b.Text() != "# New\ne\u0301" || b.Dirty || b.Version != version+1 {
		t.Fatalf("reload lost buffer identity/state: %+v", b)
	}
	if b.Row != 1 || b.Col != 2 || !e.keepOpen || e.top != 3 || e.hscroll != 5 || len(e.completions) != 0 {
		t.Fatal("reload did not preserve/clamp position or clear stale completion")
	}
	if text := e.service.Documents.Text(e.uri); text != b.Text() {
		t.Fatalf("language document is stale: %q", text)
	}
	if !reflect.DeepEqual(e.tokens, e.service.Semantic(e.uri)) {
		t.Fatal("semantic tokens are stale")
	}
	inside, delimiter, _ := e.structuralStates()
	if inside[1] || delimiter[0] {
		t.Fatal("structural highlighting cache is stale")
	}
	e.layout()
	if e.top != 1 || e.hscroll != 1 {
		t.Fatalf("viewport not clamped: top=%d hscroll=%d", e.top, e.hscroll)
	}
	if e.checkExternal() {
		t.Fatal("unchanged file triggered another reload")
	}
}

func TestExternalReloadDetectsSameSizeAndTimestamp(t *testing.T) {
	e := externalEditor(t, "old")
	info, err := os.Stat(e.buffer.Path)
	if err != nil {
		t.Fatal(err)
	}
	writeExternal(t, e.buffer.Path, "new")
	if err := os.Chtimes(e.buffer.Path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if !e.checkExternal() || e.buffer.Text() != "new" {
		t.Fatal("missed same-size write with unchanged timestamp")
	}
	// Editors commonly save by replacing the entire file.
	replacement := filepath.Join(filepath.Dir(e.buffer.Path), "replacement.md")
	writeExternal(t, replacement, "replacement")
	if err := os.Rename(replacement, e.buffer.Path); err != nil {
		t.Fatal(err)
	}
	if !e.checkExternal() || e.buffer.Text() != "replacement" {
		t.Fatal("missed atomic replacement")
	}
}

func TestExternalConflictPreservesLocalEdits(t *testing.T) {
	e := externalEditor(t, "original")
	e.buffer.InsertText("local ")
	text, version := e.buffer.Text(), e.buffer.Version
	writeExternal(t, e.buffer.Path, "external")
	if !e.checkExternal() || e.buffer.Text() != text || !e.buffer.Dirty || e.buffer.Version != version {
		t.Fatal("external change overwrote unsaved edits")
	}
	if !strings.Contains(e.statusBar(time.Now()), "changed externally; local edits kept") {
		t.Fatal("missing conflict warning")
	}
	if e.checkExternal() {
		t.Fatal("unchanged conflict should not trigger another full redraw")
	}
	// An unrelated status update must not hide an unresolved conflict.
	e.status = "another message"
	if !strings.Contains(e.statusBar(time.Now()), "changed externally") {
		t.Fatal("conflict warning was hidden")
	}
	// Saving explicitly keeps the existing overwrite behavior and resets baseline.
	e.saveWithoutExit()
	if e.externalStatus != "" || e.buffer.Dirty || e.checkExternal() {
		t.Fatal("own save was detected as an external change")
	}
	data, err := os.ReadFile(e.buffer.Path)
	if err != nil || string(data) != text {
		t.Fatalf("local save failed: %q, %v", data, err)
	}
	writeExternal(t, e.buffer.Path, "next external change")
	if !e.checkExternal() || e.buffer.Text() != "next external change" {
		t.Fatal("reload after saving failed")
	}
}

func TestExternalConflictClearsWhenDiskReverts(t *testing.T) {
	e := externalEditor(t, "original")
	e.buffer.InsertText("local ")
	writeExternal(t, e.buffer.Path, "external")
	e.checkExternal()
	writeExternal(t, e.buffer.Path, "original")
	if !e.checkExternal() || e.externalStatus != "" || !e.buffer.Dirty || e.buffer.Text() != "local original" {
		t.Fatal("reverted external edit did not clear warning while preserving local edits")
	}
}

func TestExternalInvalidOrDeletedFilePreservesBuffer(t *testing.T) {
	for _, kind := range []string{"deleted", "binary", "invalid UTF-8", "directory"} {
		t.Run(kind, func(t *testing.T) {
			e := externalEditor(t, "original")
			switch kind {
			case "deleted", "directory":
				if err := os.Remove(e.buffer.Path); err != nil {
					t.Fatal(err)
				}
				if kind == "directory" {
					if err := os.Mkdir(e.buffer.Path, 0755); err != nil {
						t.Fatal(err)
					}
				}
			case "binary":
				writeExternal(t, e.buffer.Path, "binary\x00")
			case "invalid UTF-8":
				writeExternal(t, e.buffer.Path, "\xff")
			}
			if !e.checkExternal() || e.externalStatus == "" || e.buffer.Text() != "original" || e.buffer.Dirty {
				t.Fatal("invalid/deleted file erased the buffer or omitted warning")
			}
			if e.checkExternal() {
				t.Fatal("unchanged error triggered another full redraw")
			}
			if kind == "directory" {
				if err := os.Remove(e.buffer.Path); err != nil {
					t.Fatal(err)
				}
			}
			writeExternal(t, e.buffer.Path, "restored")
			if !e.checkExternal() || e.buffer.Text() != "restored" || e.externalStatus != "" {
				t.Fatal("reload did not recover after file restoration")
			}
		})
	}
}

func TestExternalCreationAndLineEndingNormalization(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "new.md")
	e, err := New(path, root, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if e.checkExternal() {
		t.Fatal("missing new file should not warn")
	}
	writeExternal(t, path, "first\r\nsecond\r\n")
	if !e.checkExternal() || e.buffer.Text() != "first\nsecond\n" || e.buffer.Dirty {
		t.Fatal("newly created file not reloaded with normalized line endings")
	}
	writeExternal(t, path, "first\nsecond\n")
	if e.checkExternal() {
		t.Fatal("line ending change alone should not reload")
	}
	writeExternal(t, path, "")
	if !e.checkExternal() || e.buffer.Text() != "" || len(e.buffer.Lines) != 1 {
		t.Fatal("empty file was not reloaded")
	}
}

func TestExternalUntitledIsNotReloaded(t *testing.T) {
	root := t.TempDir()
	e, err := NewUntitled(root, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	e.buffer.InsertText("draft")
	writeExternal(t, filepath.Join(root, "prompt.md"), "external")
	if e.checkExternal() || e.buffer.Text() != "draft" || !e.buffer.Dirty {
		t.Fatal("untitled buffer was affected by a disk file")
	}
}

func TestExternalReloadOnReturnToParent(t *testing.T) {
	for _, dirty := range []bool{false, true} {
		t.Run(map[bool]string{false: "clean", true: "dirty"}[dirty], func(t *testing.T) {
			e := externalEditor(t, "@child.md")
			s := &editorSession{current: e}
			openSessionReference(t, s)
			if dirty {
				e.buffer.InsertText("local ")
			}
			text := e.buffer.Text()
			writeExternal(t, e.buffer.Path, "external parent")
			s.handle(Key{Kind: KeyCancel})
			s.handle(Key{Kind: KeyRune, Rune: 'y'})
			if s.current != e {
				t.Fatal("did not return to parent")
			}
			if dirty {
				if e.buffer.Text() != text || e.externalStatus == "" {
					t.Fatal("suspended parent's unsaved edits were lost or warning omitted")
				}
			} else if e.buffer.Text() != "external parent" || e.buffer.Dirty {
				t.Fatal("clean parent was not refreshed on return")
			}
		})
	}
}

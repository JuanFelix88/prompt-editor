package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestKeepOpenSavesRepeatedlyAndCanBeDisabled(t *testing.T) {
	for _, kind := range []KeyKind{KeySave, KeySaveWithoutExit} {
		t.Run(map[KeyKind]string{KeySave: "Ctrl+S", KeySaveWithoutExit: "Ctrl+Shift+S"}[kind], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "prompt.md")
			e := &Editor{buffer: &Buffer{Path: path, Lines: [][]rune{{}}}, cols: 80}
			if _, _, exit := e.handle(Key{Kind: KeySave}); exit != 1 {
				t.Fatal("default save must request exit")
			}
			changed, complete, exit := e.handle(Key{Kind: KeyToggleKeepOpen})
			if changed || complete || exit != 0 || !e.keepOpen {
				t.Fatal("toggle must enable Keep open without changing the document or exiting")
			}
			for _, text := range []string{"first", " second"} {
				e.buffer.InsertText(text)
				changed, complete, exit = e.handle(Key{Kind: kind})
				if changed || complete || exit != 0 || e.buffer.Dirty {
					t.Fatalf("save: changed=%v complete=%v exit=%d dirty=%v", changed, complete, exit, e.buffer.Dirty)
				}
				data, err := os.ReadFile(path)
				if err != nil || string(data) != e.buffer.Text() {
					t.Fatalf("save: data=%q err=%v", data, err)
				}
				if !strings.Contains(e.statusBar(time.Now()), "Ctrl+K Keep open: ON") {
					t.Fatal("mode and shortcut must remain visible after saving")
				}
			}
			e.handle(Key{Kind: KeyToggleKeepOpen})
			if e.keepOpen || !strings.Contains(e.statusBar(time.Now()), "Ctrl+K Keep open: OFF") {
				t.Fatal("toggle must disable Keep open and update the status bar")
			}
			if _, _, exit := e.handle(Key{Kind: KeySave}); exit != 1 {
				t.Fatal("save must request exit after disabling Keep open")
			}
		})
	}
}

func TestKeepOpenUntitledDoesNotConfirmOrWrite(t *testing.T) {
	root := t.TempDir()
	e, err := NewUntitled(root, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	e.buffer.InsertText("draft")
	e.handle(Key{Kind: KeyToggleKeepOpen})
	for _, kind := range []KeyKind{KeySave, KeySaveWithoutExit} {
		if _, _, exit := e.handle(Key{Kind: kind}); exit != 0 {
			t.Fatal("untitled save must not exit while Keep open is enabled")
		}
		if e.buffer.Text() != "draft" || !e.buffer.Dirty || !strings.Contains(e.status, "Ctrl+K") {
			t.Fatalf("untitled buffer changed or missing guidance: %q", e.status)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("unexpected disk writes: entries=%v err=%v", entries, err)
	}
	e.handle(Key{Kind: KeyToggleKeepOpen})
	if _, _, exit := e.handle(Key{Kind: KeySave}); exit != 1 {
		t.Fatal("untitled save must request confirmation after disabling Keep open")
	}
	if text, done := e.confirm(); !done || text != "draft" {
		t.Fatalf("confirm: text=%q done=%v", text, done)
	}
}

func TestKeepOpenSaveFailurePreservesBuffer(t *testing.T) {
	e := &Editor{buffer: &Buffer{Path: filepath.Join(t.TempDir(), "missing", "prompt.md"), Lines: [][]rune{[]rune("draft")}, Dirty: true}, keepOpen: true}
	if _, _, exit := e.handle(Key{Kind: KeySave}); exit != 0 {
		t.Fatal("failed save must not exit")
	}
	if !e.buffer.Dirty || e.buffer.Text() != "draft" || !strings.HasPrefix(e.status, "save failed:") {
		t.Fatalf("failed save: dirty=%v status=%q", e.buffer.Dirty, e.status)
	}
}

func TestKeepOpenDoesNotPreventExplicitCancel(t *testing.T) {
	e := &Editor{buffer: &Buffer{Lines: [][]rune{{}}}, keepOpen: true}
	e.handle(Key{Kind: KeyCancel})
	if !e.confirmCancel {
		t.Fatal("cancel must still ask for confirmation")
	}
	if _, _, exit := e.handle(Key{Kind: KeyRune, Rune: 'y'}); exit != -1 {
		t.Fatal("confirmed cancellation must exit")
	}
}

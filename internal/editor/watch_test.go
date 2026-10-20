package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func testFileWatch(t *testing.T, e *Editor) *fileWatch {
	t.Helper()
	w, err := newFileWatch(e)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.Close() })
	return w
}

// Wait for real OS notifications; no polling or direct reload before an event.
func waitExternalEvent(t *testing.T, w *fileWatch, e *Editor, done func() bool) {
	t.Helper()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case event := <-w.Events:
			if w.relevant(event) {
				if err := w.follow(e); err != nil {
					t.Fatal(err)
				}
				e.checkExternal()
				if done() {
					return
				}
			}
		case err := <-w.Errors:
			t.Fatal(err)
		case <-timeout.C:
			t.Fatalf("no matching file notification; text=%q warning=%q", e.buffer.Text(), e.externalStatus)
		}
	}
}

func TestFileWatchNativeWritesAndAtomicReplacement(t *testing.T) {
	e := externalEditor(t, "original")
	w := testFileWatch(t, e)
	writeExternal(t, e.buffer.Path, "external")
	waitExternalEvent(t, w, e, func() bool { return e.buffer.Text() == "external" })

	tmp := filepath.Join(filepath.Dir(e.buffer.Path), "replacement")
	writeExternal(t, tmp, "replaced")
	if err := os.Rename(tmp, e.buffer.Path); err != nil {
		t.Fatal(err)
	}
	waitExternalEvent(t, w, e, func() bool { return e.buffer.Text() == "replaced" })

	// Replacement must not detach the watch from future changes.
	writeExternal(t, e.buffer.Path, "after replacement")
	waitExternalEvent(t, w, e, func() bool { return e.buffer.Text() == "after replacement" })

	e.buffer.InsertText("local ")
	writeExternal(t, e.buffer.Path, "conflicting")
	waitExternalEvent(t, w, e, func() bool { return strings.Contains(e.externalStatus, "changed externally") })
	if e.buffer.Text() != "local after replacement" || !e.buffer.Dirty {
		t.Fatal("native notification overwrote local edits")
	}
}

func TestFileWatchFollowsActiveEditorAndFiltersEvents(t *testing.T) {
	e := externalEditor(t, "parent")
	w := testFileWatch(t, e)
	for _, name := range []string{"other.md", "prompt.md.tmp", "prompt.md/child"} {
		if w.relevant(fsnotify.Event{Name: filepath.Join(filepath.Dir(e.buffer.Path), name), Op: fsnotify.Write}) {
			t.Fatalf("unrelated event matched: %s", name)
		}
	}
	if !w.relevant(fsnotify.Event{Name: e.buffer.Path, Op: fsnotify.Write}) {
		t.Fatal("target file event was ignored")
	}
	child := externalEditor(t, "child")
	if err := w.follow(child); err != nil {
		t.Fatal(err)
	}
	if watches := w.WatchList(); len(watches) != 1 || watches[0] != filepath.Dir(child.buffer.Path) {
		t.Fatalf("watch did not follow active directory: %v", watches)
	}
	if w.relevant(fsnotify.Event{Name: e.buffer.Path, Op: fsnotify.Write}) {
		t.Fatal("suspended parent still matches events")
	}
	writeExternal(t, child.buffer.Path, "changed child")
	waitExternalEvent(t, w, child, func() bool { return child.buffer.Text() == "changed child" })
	if err := w.follow(e); err != nil {
		t.Fatal(err)
	}
	writeExternal(t, e.buffer.Path, "changed parent")
	waitExternalEvent(t, w, e, func() bool { return e.buffer.Text() == "changed parent" })

	untitled, err := NewUntitled(filepath.Dir(e.buffer.Path), 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.follow(untitled); err != nil {
		t.Fatal(err)
	}
	if len(w.WatchList()) != 0 || w.relevant(fsnotify.Event{Name: e.buffer.Path, Op: fsnotify.Write}) {
		t.Fatal("untitled buffer should not watch a file")
	}
}

func TestFileWatchMissingParentDirectories(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "missing", "nested", "new.md")
	e, err := New(path, root, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	w := testFileWatch(t, e)
	if w.directory != root {
		t.Fatalf("not watching nearest ancestor: %q", w.directory)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	writeExternal(t, path, "created externally")
	waitExternalEvent(t, w, e, func() bool { return e.buffer.Text() == "created externally" })
	if w.directory != filepath.Dir(path) {
		t.Fatal("watch did not move to the newly created directory")
	}
	writeExternal(t, path, "changed again")
	waitExternalEvent(t, w, e, func() bool { return e.buffer.Text() == "changed again" })
}

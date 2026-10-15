package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/juanandrade/prompt-cli/internal/lang"
)

func referenceSession(t *testing.T, root, name, text string) *editorSession {
	t.Helper()
	var e *Editor
	var err error
	if name == "" {
		e, err = NewUntitled(root, 1, 1)
	} else {
		e, err = New(filepath.Join(root, name), root, 1, 1)
	}
	if err != nil {
		t.Fatal(err)
	}
	e.buffer.InsertText(text)
	return &editorSession{current: e}
}

func openSessionReference(t *testing.T, s *editorSession) *Editor {
	t.Helper()
	before := s.current
	changed, complete, exit := s.handle(Key{Kind: KeyOpenReference})
	if changed || complete || exit != 0 || s.current == before {
		t.Fatalf("reference did not open: changed=%v complete=%v exit=%d status=%s", changed, complete, exit, before.status)
	}
	return s.current
}

func TestOpenReferenceCaretBoundaries(t *testing.T) {
	root := t.TempDir()
	text := "😀 read @new-file.md next"
	start := len([]rune("😀 read "))
	end := start + len([]rune("@new-file.md"))
	for col := 0; col <= len([]rune(text)); col++ {
		s := referenceSession(t, root, "prompt.md", text)
		parent := s.current
		parent.buffer.Col = col
		s.handle(Key{Kind: KeyOpenReference})
		want := col >= start && col <= end
		if got := s.current != parent; got != want {
			t.Fatalf("col=%d opened=%v want=%v", col, got, want)
		}
		if want && s.current.buffer.Path != filepath.Join(root, "new-file.md") {
			t.Fatalf("wrong target: %s", s.current.buffer.Path)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "new-file.md")); !os.IsNotExist(err) {
		t.Fatalf("opening created file: %v", err)
	}
}

func TestReferenceSessionSaveRestoresParent(t *testing.T) {
	root := t.TempDir()
	s := referenceSession(t, root, "prompt.md", "@new.txt")
	parent := s.current
	parent.top, parent.hscroll, parent.keepOpen = 2, 4, true
	parent.openedAt = time.Now().Add(-time.Minute)
	parent.output = os.Stderr
	parent.refreshCompletion()
	text, row, col, version, openedAt := parent.buffer.Text(), parent.buffer.Row, parent.buffer.Col, parent.buffer.Version, parent.openedAt
	child := openSessionReference(t, s)
	if !child.plainText || child.output != parent.output || child.keepOpen || child.anonymous || child.openedAt.IsZero() {
		t.Fatal("child did not get independent file editor state")
	}
	child.buffer.InsertText("hello 世界\n")
	if _, _, exit := s.handle(Key{Kind: KeySave}); exit != 0 || s.current != parent {
		t.Fatal("saving child must return to parent, not exit the session")
	}
	data, err := os.ReadFile(filepath.Join(root, "new.txt"))
	if err != nil || string(data) != "hello 世界\n" {
		t.Fatalf("save: %q, %v", data, err)
	}
	if parent.buffer.Text() != text || parent.buffer.Row != row || parent.buffer.Col != col || parent.buffer.Version != version || !parent.buffer.Dirty || parent.top != 2 || parent.hscroll != 4 || !parent.keepOpen || parent.openedAt != openedAt {
		t.Fatal("parent editing state was lost")
	}
	if _, err := os.Stat(parent.buffer.Path); !os.IsNotExist(err) {
		t.Fatal("parent was saved implicitly")
	}
	if len(parent.tokens) != 1 || parent.tokens[0].Kind != lang.TokenFile {
		t.Fatalf("parent missing-reference style not refreshed: %#v", parent.tokens)
	}
}

func TestReferenceExistingTextAndNestedNavigation(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "sub", "script.go")
	if err := os.WriteFile(path, []byte("@../other.json"), 0644); err != nil {
		t.Fatal(err)
	}
	// Explicit navigation also works in ordinary Markdown and plain-text files.
	s := referenceSession(t, root, "notes.md", "@sub/script.go")
	parent := s.current
	child := openSessionReference(t, s)
	if child.buffer.Text() != "@../other.json" || !child.plainText {
		t.Fatalf("existing content=%q", child.buffer.Text())
	}
	grandchild := openSessionReference(t, s)
	if grandchild.buffer.Path != filepath.Join(root, "other.json") {
		t.Fatalf("nested reference base: %s", grandchild.buffer.Path)
	}
	grandchild.buffer.InsertText("{}")
	s.handle(Key{Kind: KeyCancel})
	if _, _, exit := s.handle(Key{Kind: KeyRune, Rune: 'y'}); exit != 0 || s.current != child {
		t.Fatal("cancel did not return to immediate parent")
	}
	if _, err := os.Stat(grandchild.buffer.Path); !os.IsNotExist(err) {
		t.Fatal("cancel created file")
	}
	s.handle(Key{Kind: KeyCancel})
	s.handle(Key{Kind: KeyRune, Rune: 'y'})
	if s.current != parent || len(s.parents) != 0 {
		t.Fatal("nested stack did not unwind")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "@../other.json" {
		t.Fatal("cancel changed existing file")
	}
	s.handle(Key{Kind: KeyCancel})
	if _, _, exit := s.handle(Key{Kind: KeyRune, Rune: 'y'}); exit != -1 {
		t.Fatal("root cancellation must still exit")
	}
}

func TestReferenceUntitledOutputOnlyOnRootConfirmation(t *testing.T) {
	root := t.TempDir()
	s := referenceSession(t, root, "", "@data.txt")
	parent := s.current
	child := openSessionReference(t, s)
	if child.buffer.Path != filepath.Join(root, "data.txt") {
		t.Fatal("untitled did not use workspace")
	}
	child.buffer.InsertText("data")
	s.handle(Key{Kind: KeySave})
	if s.current != parent || s.result != "" {
		t.Fatal("child must not produce stdout")
	}
	if _, _, exit := s.handle(Key{Kind: KeySave}); exit != 1 || s.result != "@data.txt" {
		t.Fatal("root stdout result lost")
	}
}

func TestReferenceSaveWithoutExitAndFailure(t *testing.T) {
	root := t.TempDir()
	s := referenceSession(t, root, "prompt.md", "@missing/new.txt")
	child := openSessionReference(t, s)
	child.buffer.InsertText("text")
	if _, _, exit := s.handle(Key{Kind: KeySave}); exit != 0 || s.current != child || !child.buffer.Dirty || !strings.Contains(child.status, "save failed") {
		t.Fatal("failed save must keep child open")
	}
	if err := os.Mkdir(filepath.Join(root, "missing"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, key := range []KeyKind{KeySaveWithoutExit, KeyToggleKeepOpen, KeySave} {
		if _, _, exit := s.handle(Key{Kind: key}); exit != 0 || s.current != child {
			t.Fatal("save-without-exit or Keep open closed child")
		}
	}
	if child.buffer.Dirty {
		t.Fatal("child not saved")
	}
	s.handle(Key{Kind: KeyToggleKeepOpen})
	s.handle(Key{Kind: KeySave})
	if s.current == child {
		t.Fatal("child did not close after disabling Keep open")
	}
}

func TestReferenceOpenErrorsAndModals(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "folder"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"binary": {0, 1}, "invalid.txt": {0xff}} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, ref := range []string{"@folder", "@folder/", "@missing/", "@binary", "@invalid.txt", "@prompt.md", "@", "[Heading]", "user@example.com"} {
		t.Run(ref, func(t *testing.T) {
			s := referenceSession(t, root, "prompt.md", ref)
			parent := s.current
			s.handle(Key{Kind: KeyOpenReference})
			if s.current != parent || len(s.parents) != 0 || !strings.Contains(parent.status, "open failed") || parent.buffer.Text() != ref {
				t.Fatalf("open should fail without modifying parent: %s", parent.status)
			}
		})
	}
	for _, modal := range []KeyKind{KeyGoTo, KeyCancel} {
		s := referenceSession(t, root, "prompt.md", "@new.md")
		parent := s.current
		s.handle(Key{Kind: modal})
		s.handle(Key{Kind: KeyOpenReference})
		if s.current != parent || len(s.parents) != 0 {
			t.Fatal("navigation escaped modal")
		}
	}
}

func TestReferenceRejectsSuspendedFile(t *testing.T) {
	root := t.TempDir()
	s := referenceSession(t, root, "prompt.md", "@child.prompt")
	child := openSessionReference(t, s)
	child.buffer.InsertText("@prompt.md")
	s.handle(Key{Kind: KeyOpenReference})
	if s.current != child || len(s.parents) != 1 || !strings.Contains(child.status, "already open") {
		t.Fatal("duplicate writable parent buffer allowed")
	}
}

func TestReferencedPlainTextHasNoMarkdownStyles(t *testing.T) {
	s := referenceSession(t, t.TempDir(), "prompt.md", "@plain.txt")
	child := openSessionReference(t, s)
	child.buffer.InsertText("# Heading | **text**")
	child.service.Documents.Change(child.uri, child.buffer.Text())
	child.tokens = child.service.Semantic(child.uri)
	for _, style := range child.stylesFor(0, child.buffer.Lines[0], false, false, false) {
		if style != "" {
			t.Fatal("Markdown highlighting applied to plain text")
		}
	}
}
func TestReferenceShortcutsWithCompletionOpen(t *testing.T) {
	for _, sequence := range []string{"\x1b[24~", "\x0f", "\x1b[111;5u"} {
		s := referenceSession(t, t.TempDir(), "prompt.md", "@new.txt")
		parent := s.current
		parent.completions = []lang.Completion{{Label: "other.txt"}}
		decoder := new(inputDecoder)
		// Feed one byte at a time, as terminals may fragment escape sequences.
		for _, b := range []byte(sequence) {
			for _, key := range decoder.Feed([]byte{b}) {
				s.handle(key)
			}
		}
		if s.current == parent || filepath.Base(s.current.buffer.Path) != "new.txt" {
			t.Fatalf("%q did not navigate past completion popup", sequence)
		}
		for _, key := range decoder.Feed([]byte("\x18y")) {
			s.handle(key)
		}
		if s.current != parent || len(parent.completions) != 1 || parent.buffer.Text() != "@new.txt" {
			t.Fatal("parent popup or text not preserved")
		}
	}
}

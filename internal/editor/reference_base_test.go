package editor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/juanandrade/prompt-cli/internal/lang"
)

func TestEditorReferencesUseFileDirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(root, "root.md"), filepath.Join(dir, "local.md")} {
		if err := os.WriteFile(path, []byte("fixture"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, path, insert string
	}{
		{"existing", filepath.Join(dir, "existing.prompt"), "local.md"},
		{"new-file", filepath.Join(dir, "new.prompt"), "local.md"},
		{"untitled", "", "root.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var e *Editor
			var err error
			if tc.name == "existing" {
				if err = os.WriteFile(tc.path, nil, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.path == "" {
				e, err = NewUntitled(root, 1, 1)
			} else {
				e, err = New(tc.path, root, 1, 1)
			}
			if err != nil {
				t.Fatal(err)
			}
			e.buffer.InsertText("@")
			e.refreshCompletion()
			found := false
			for i, item := range e.completions {
				if item.TextEdit.NewText == tc.insert {
					found = true
					e.selected = i
					break
				}
			}
			if !found {
				t.Fatalf("missing %q: %#v", tc.insert, e.completions)
			}
			e.accept()
			if text := e.buffer.Text(); text != "@"+tc.insert {
				t.Fatalf("accepted completion=%q", text)
			}
			e.service.Documents.Change(e.uri, e.buffer.Text())
			tokens := e.service.SemanticCached(e.uri)
			if len(tokens) != 1 || tokens[0].Kind != lang.TokenFile {
				t.Fatalf("reference tokens=%#v", tokens)
			}
		})
	}
}

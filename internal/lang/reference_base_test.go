package lang

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeReferenceFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fixture"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestReferencesUseEachDocumentDirectory(t *testing.T) {
	root := t.TempDir()
	writeReferenceFixture(t, filepath.Join(root, "guide.md"))
	writeReferenceFixture(t, filepath.Join(root, "a", "guide.md"))
	writeReferenceFixture(t, filepath.Join(root, "b", "other.md"))
	outside := t.TempDir()
	writeReferenceFixture(t, filepath.Join(outside, "guide.md"))
	s, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, dir string
		exists    bool
	}{
		{"nested", filepath.Join(root, "a"), true},
		{"different-document", filepath.Join(root, "b"), false},
		{"outside-workspace", outside, true},
		{"cached-nested-again", filepath.Join(root, "a"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uri := PathToURI(filepath.Join(tc.dir, "unsaved.prompt"))
			s.Documents.Open(uri, "@guide.md")
			completion := s.Complete(uri, Position{Character: 3})
			found := false
			for _, item := range completion.Items {
				if item.TextEdit.NewText == "guide.md" {
					found = true
				}
				if strings.Contains(item.TextEdit.NewText, "/") {
					t.Errorf("candidate from another directory: %q", item.TextEdit.NewText)
				}
			}
			if found != tc.exists {
				t.Errorf("completion found=%v want %v: %#v", found, tc.exists, completion)
			}
			assertReferenceTarget(t, s, uri, filepath.Join(tc.dir, "guide.md"), tc.exists)
		})
	}
}

func assertReferenceTarget(t *testing.T, s *Service, uri, target string, exists bool) {
	t.Helper()
	pos := Position{Character: 2}
	definition := s.Definition(uri, pos)
	if exists {
		if definition == nil || definition.URI != PathToURI(target) {
			t.Errorf("definition=%#v want %s", definition, target)
		}
	} else if definition != nil {
		t.Errorf("missing reference definition=%#v", definition)
	}
	hover := s.Hover(uri, pos)
	if hover == nil {
		t.Fatal("missing hover")
	}
	if exists && !strings.Contains(hover.Contents.Value, target) {
		t.Errorf("hover=%q want target %q", hover.Contents.Value, target)
	}
	if !exists && !strings.Contains(hover.Contents.Value, "Missing file or folder") {
		t.Errorf("missing reference hover=%q", hover.Contents.Value)
	}
	wantKind := TokenFileMissing
	if exists {
		wantKind = TokenFile
	}
	for _, semantic := range []func(string) []SemanticToken{s.Semantic, s.SemanticCached} {
		tokens := semantic(uri)
		if len(tokens) != 1 || tokens[0].Kind != wantKind {
			t.Errorf("tokens=%#v want one token of kind %v", tokens, wantKind)
		}
	}
}

func TestRelativePathsAndParentCompletion(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	dir := filepath.Join(workspace, "docs")
	for _, path := range []string{
		filepath.Join(dir, "guide.md"),
		filepath.Join(dir, "src", "main.go"),
		filepath.Join(workspace, "parent.md"),
		filepath.Join(workspace, "assets", "image.md"),
		filepath.Join(root, "outside.md"),
	} {
		writeReferenceFixture(t, path)
	}
	s, err := NewService(workspace)
	if err != nil {
		t.Fatal(err)
	}
	uri := PathToURI(filepath.Join(dir, "prompt.md"))
	for _, tc := range []struct {
		prefix, insert, target string
	}{
		{"gu", "guide.md", filepath.Join(dir, "guide.md")},
		{"./gu", "./guide.md", filepath.Join(dir, "guide.md")},
		{"/gu", "guide.md", filepath.Join(dir, "guide.md")},
		{"SRC/m", "src/main.go", filepath.Join(dir, "src", "main.go")},
		{"../pa", "../parent.md", filepath.Join(workspace, "parent.md")},
		{"../../ou", "../../outside.md", filepath.Join(root, "outside.md")},
		{"../assets/", "../assets/image.md", filepath.Join(workspace, "assets", "image.md")},
		{"src/../gu", "src/../guide.md", filepath.Join(dir, "guide.md")},
		{"../as", "../assets/", filepath.Join(workspace, "assets")},
	} {
		t.Run(tc.prefix, func(t *testing.T) {
			text := "See @" + tc.prefix
			s.Documents.Open(uri, text)
			pos := Position{Character: UTF16Len(text)}
			completion := s.Complete(uri, pos)
			found := false
			for _, item := range completion.Items {
				if item.TextEdit.NewText == tc.insert {
					found = true
					if item.TextEdit.Range.Start.Character != 5 || item.TextEdit.Range.End != pos {
						t.Errorf("replacement range=%#v", item.TextEdit.Range)
					}
				}
			}
			if !found {
				t.Fatalf("missing %q in %#v", tc.insert, completion)
			}
			s.Documents.Change(uri, "@"+tc.insert)
			assertReferenceTarget(t, s, uri, tc.target, true)
		})
	}
}

func TestReferenceResolutionRemainsFresh(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs")
	target := filepath.Join(dir, "guide.md")
	writeReferenceFixture(t, target)
	s, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	uri := PathToURI(filepath.Join(dir, "prompt.md"))
	s.Documents.Open(uri, "@guide.md")
	assertReferenceTarget(t, s, uri, target, true)
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if s.Definition(uri, Position{Character: 2}) != nil {
		t.Fatal("deleted target still has a definition")
	}
	if h := s.Hover(uri, Position{Character: 2}); h == nil || !strings.Contains(h.Contents.Value, "Missing") {
		t.Fatalf("deleted target hover=%#v", h)
	}
	if tokens := s.Semantic(uri); len(tokens) != 1 || tokens[0].Kind != TokenFileMissing {
		t.Fatalf("deleted target tokens=%#v", tokens)
	}
}

func TestResolveAllowsParentsButRejectsInvalidPaths(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "workspace")
	if err := os.Mkdir(base, 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "outside.md")
	writeReferenceFixture(t, target)
	if path, _, ok := Resolve(base, "../outside.md"); !ok || path != target {
		t.Fatalf("parent resolved to %q, exists=%v", path, ok)
	}
	invalid := []string{"bad\x00path"}
	if runtime.GOOS == "windows" {
		invalid = append(invalid, "C:/outside.md", "C:outside.md")
	}
	for _, value := range invalid {
		if _, _, ok := Resolve(base, value); ok {
			t.Errorf("invalid path resolved: %q", value)
		}
	}
}

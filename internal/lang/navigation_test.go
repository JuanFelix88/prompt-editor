package lang

import (
	"path/filepath"
	"testing"
)

func TestFileReferenceTargetWithoutExistenceOrPromptActivation(t *testing.T) {
	root := t.TempDir()
	s, err := NewService(filepath.Join(root, "workspace"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"prompt.md", "notes.md", "script.go"} {
		uri := PathToURI(filepath.Join(root, "docs", name))
		s.Documents.Open(uri, "😀 @../new.json @other.txt\n[Heading]")
		for _, tc := range []struct {
			pos  Position
			want string
		}{
			{Position{0, 2}, ""},
			{Position{0, 3}, filepath.Join(root, "new.json")},
			{Position{0, 9}, filepath.Join(root, "new.json")},
			{Position{0, 15}, filepath.Join(root, "new.json")},
			{Position{0, 16}, filepath.Join(root, "docs", "other.txt")},
			{Position{0, 26}, filepath.Join(root, "docs", "other.txt")},
			{Position{0, 27}, ""},
			{Position{1, 3}, ""},
			{Position{-1, 0}, ""},
			{Position{2, 0}, ""},
		} {
			got, ok := s.FileReferenceTarget(uri, tc.pos)
			if got != tc.want || ok != (tc.want != "") {
				t.Errorf("%s pos=%v: got %q, %v; want %q", name, tc.pos, got, ok, tc.want)
			}
		}
		if got := s.Definition(uri, Position{0, 9}); got != nil {
			t.Errorf("LSP must not navigate to missing files: %#v", got)
		}
	}
}

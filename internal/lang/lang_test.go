package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanHeadingsHierarchyAndFences(t *testing.T) {
	text := "# Parent\n```go\n## Hidden\n```\n### Child ###\n## Peer\n"
	hs := ScanHeadings(text)
	if len(hs) != 3 {
		t.Fatalf("got %d headings: %#v", len(hs), hs)
	}
	want := []string{"Parent", "Parent > Child", "Parent > Peer"}
	for i, h := range hs {
		if h.Path != want[i] {
			t.Errorf("heading %d path=%q want %q", i, h.Path, want[i])
		}
	}
}
func TestReferencesRulesAndUTF16(t *testing.T) {
	refs := ScanReferences("😀 mail@example.com @src/a.go [Parent > Child] [x] ![img] [link](url)")
	if len(refs) != 2 {
		t.Fatalf("refs=%#v", refs)
	}
	if !refs[0].File || refs[0].Value != "src/a.go" {
		t.Fatalf("file ref=%#v", refs[0])
	}
	if refs[0].StartUTF16 != 20 {
		t.Fatalf("UTF-16 start=%d want 20", refs[0].StartUTF16)
	}
	if refs[1].File || refs[1].Value != "Parent > Child" {
		t.Fatalf("heading ref=%#v", refs[1])
	}
}
func TestCompletionAndFreshResolution(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "docs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "guide.md"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "my.prompt.md")
	uri := PathToURI(path)
	s, _ := NewService(root)
	s.Documents.Open(uri, "See @do\n# Parent\n## Child\nUse [par")
	files := s.Complete(uri, Position{Line: 0, Character: 7})
	if len(files.Items) == 0 || files.Items[0].TextEdit.NewText != "docs/" {
		t.Fatalf("file completion=%#v", files)
	}
	headings := s.Complete(uri, Position{Line: 3, Character: 8})
	if len(headings.Items) != 2 || headings.Items[0].TextEdit.NewText != "Parent]" {
		t.Fatalf("heading completion=%#v", headings)
	}
	h := s.Hover(uri, Position{Line: 0, Character: 6})
	if h == nil || !strings.Contains(h.Contents.Value, "folder") {
		t.Fatalf("hover=%#v", h)
	}
}
func TestTraversalAndMarkdownActivation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "guide.md"), []byte("# Guide"), 0644); err != nil {
		t.Fatal(err)
	}
	s, _ := NewService(root)
	if _, _, ok := Resolve(root, "../outside"); ok {
		t.Fatal("workspace traversal resolved")
	}
	uri := PathToURI(filepath.Join(root, "notes.md"))
	s.Documents.Open(uri, "@gu\n# H")
	if got := s.Complete(uri, Position{Line: 0, Character: 3}); len(got.Items) == 0 || got.Items[0].TextEdit.NewText != "guide.md" {
		t.Fatalf("markdown completion=%#v", got)
	}
	if tokens := s.Semantic(uri); len(tokens) == 0 {
		t.Fatal("markdown got no semantic tokens")
	}
}
func TestSemanticTokensDoNotOverlap(t *testing.T) {
	root := t.TempDir()
	s, _ := NewService(root)
	uri := PathToURI(filepath.Join(root, "prompt.md"))
	s.Documents.Open(uri, "# Read @missing and [Read @missing and]\n- use @missing")
	tokens := s.Semantic(uri)
	lastLine, lastEnd := -1, 0
	for _, tok := range tokens {
		if tok.Line == lastLine && tok.Start < lastEnd {
			t.Fatalf("overlap after %d: %#v", lastEnd, tok)
		}
		lastLine = tok.Line
		lastEnd = tok.Start + tok.Length
	}
	data := EncodeSemantic(tokens)
	if len(data) != len(tokens)*5 {
		t.Fatalf("encoded size=%d", len(data))
	}
}
func TestOpenSnapshotWinsOverDisk(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "prompt.md")
	if err := os.WriteFile(path, []byte("# Disk"), 0644); err != nil {
		t.Fatal(err)
	}
	s, _ := NewService(root)
	uri := PathToURI(path)
	s.Documents.Open(uri, "# Unsaved\n[Unsaved]")
	loc := s.Definition(uri, Position{Line: 1, Character: 2})
	if loc == nil || loc.Range.Start.Line != 0 {
		t.Fatalf("definition=%#v", loc)
	}
}

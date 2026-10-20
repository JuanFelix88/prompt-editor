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
func TestMarkdownActivation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "guide.md"), []byte("# Guide"), 0644); err != nil {
		t.Fatal(err)
	}
	s, _ := NewService(root)
	uri := PathToURI(filepath.Join(root, "notes.md"))
	s.Documents.Open(uri, "@gu\n# H")
	if got := s.Complete(uri, Position{Line: 0, Character: 3}); len(got.Items) == 0 || got.Items[0].TextEdit.NewText != "guide.md" {
		t.Fatalf("markdown completion=%#v", got)
	}
	if tokens := s.Semantic(uri); len(tokens) == 0 {
		t.Fatal("markdown got no semantic tokens")
	}
}
func TestPromptItemSemanticTokens(t *testing.T) {
	root := t.TempDir()
	s, _ := NewService(root)
	uri := PathToURI(filepath.Join(root, "prompt.md"))
	s.Documents.Open(uri, "- red\n  * blue\n+ structural only\n*not-an-item")

	tokens := s.Semantic(uri)
	if len(tokens) != 2 {
		t.Fatalf("tokens=%#v", tokens)
	}
	if tokens[0].Line != 0 || tokens[0].Start != 0 || tokens[0].Length != 5 || tokens[0].Kind != TokenPromptItem {
		t.Errorf("hyphen item token=%#v", tokens[0])
	}
	if tokens[1].Line != 1 || tokens[1].Start != 2 || tokens[1].Length != 6 || tokens[1].Kind != TokenPromptStarItem {
		t.Errorf("star item token=%#v", tokens[1])
	}

	s.Documents.Change(uri, "* use @missing")
	tokens = s.Semantic(uri)
	if len(tokens) != 2 || tokens[0].Kind != TokenPromptStarItem || tokens[0].Start != 0 || tokens[0].Length != 6 || tokens[1].Kind != TokenFileMissing || tokens[1].Start != 6 {
		t.Fatalf("star item with reference tokens=%#v", tokens)
	}
}

func TestPromptStarItemSemanticEncoding(t *testing.T) {
	if TokenPromptStarItem != TokenPromptItem+1 || PlainTokenName(TokenPromptStarItem) != "promptStarItem" {
		t.Fatalf("star item legend index=%d name=%q", TokenPromptStarItem, PlainTokenName(TokenPromptStarItem))
	}
	data := EncodeSemantic([]SemanticToken{{Line: 0, Start: 2, Length: 4, Kind: TokenPromptStarItem}})
	if len(data) != 5 || data[0] != 0 || data[1] != 2 || data[2] != 4 || data[3] != int(TokenPromptStarItem) || data[4] != 0 {
		t.Fatalf("encoded star item=%v", data)
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
func TestTaskMarkerSemanticTokens(t *testing.T) {
	root := t.TempDir()
	s, _ := NewService(root)
	uri := PathToURI(filepath.Join(root, "prompt.md"))
	s.Documents.Open(uri, "- [x] done\n* [*] custom\n+ [X] upper\n- [ ] pending\nsee [x] inline")

	// A marker covers its inner character only, never the brackets, and never
	// a bracketed value away from the list marker.
	want := []SemanticToken{
		{Line: 0, Start: 3, Length: 1, Kind: TokenTaskDone},
		{Line: 1, Start: 3, Length: 1, Kind: TokenTaskMarker},
		{Line: 2, Start: 3, Length: 1, Kind: TokenTaskDone},
	}
	var got []SemanticToken
	for _, tok := range s.Semantic(uri) {
		if tok.Kind == TokenTaskDone || tok.Kind == TokenTaskMarker {
			got = append(got, tok)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("markers=%#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("marker %d=%#v want %#v", i, got[i], want[i])
		}
	}
}

// A task marker must not be resolved as a heading reference: `[*]` used to be
// painted as a missing reference because only `x` was excluded.
func TestTaskMarkerIsNotAHeadingReference(t *testing.T) {
	root := t.TempDir()
	s, _ := NewService(root)
	uri := PathToURI(filepath.Join(root, "prompt.md"))
	s.Documents.Open(uri, "- [*] custom\n- [x] done and [Real]\n# Real")

	tokens := s.Semantic(uri)
	// The item is split around the marker instead of overlapping it, and the
	// real `[Real]` reference on the same line still resolves.
	want := []SemanticToken{
		{Line: 0, Start: 0, Length: 3, Kind: TokenPromptItem},
		{Line: 0, Start: 3, Length: 1, Kind: TokenTaskMarker},
		{Line: 0, Start: 4, Length: 8, Kind: TokenPromptItem},
		{Line: 1, Start: 0, Length: 3, Kind: TokenPromptItem},
		{Line: 1, Start: 3, Length: 1, Kind: TokenTaskDone},
		{Line: 1, Start: 4, Length: 11, Kind: TokenPromptItem},
		{Line: 1, Start: 15, Length: 6, Kind: TokenHeadingRef},
		{Line: 2, Start: 0, Length: 1, Kind: TokenHeadingMarker},
		{Line: 2, Start: 2, Length: 4, Kind: TokenHeading1},
	}
	if len(tokens) != len(want) {
		t.Fatalf("tokens=%#v want %#v", tokens, want)
	}
	for i := range want {
		if tokens[i] != want[i] {
			t.Errorf("token %d=%#v want %#v", i, tokens[i], want[i])
		}
	}
	for _, tok := range tokens {
		if tok.Kind == TokenHeadingRefMissing {
			t.Fatalf("task marker resolved as a missing reference: %#v", tok)
		}
	}
}

func TestScanTaskMarkerBoundaries(t *testing.T) {
	for _, line := range []string{"-[*] no space", "see [x] inline", "- [ ] unchecked", "- [xy] not a marker", "- [x]y no gap", "text - [x]", "1. [x] ordered"} {
		if _, ok := ScanTaskMarker(line); ok {
			t.Errorf("unexpected task marker in %q", line)
		}
	}
	for _, line := range []string{"- [x] ok", "  * [X] ok", "+\t[*] ok", "- [x]", "- [*]", "- [😀] astral"} {
		if _, ok := ScanTaskMarker(line); !ok {
			t.Errorf("missed task marker in %q", line)
		}
	}
	if m, ok := ScanTaskMarker("- [x] ok"); !ok || m.Value != "x" || m.StartRune != 3 || m.StartUTF16 != 3 || m.EndUTF16 != 4 {
		t.Errorf("done marker=%#v ok=%v", m, ok)
	}
	// An astral marker occupies one rune but two UTF-16 code units.
	if m, ok := ScanTaskMarker("- [😀] ok"); !ok || m.StartRune != 3 || m.EndRune != 4 || m.StartUTF16 != 3 || m.EndUTF16 != 5 {
		t.Errorf("astral marker=%#v ok=%v", m, ok)
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

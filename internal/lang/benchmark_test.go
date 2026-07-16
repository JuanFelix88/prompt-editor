package lang

import (
	"path/filepath"
	"strings"
	"testing"
)

func BenchmarkSemanticCachedLargePrompt(b *testing.B) {
	root := b.TempDir()
	s, _ := NewService(root)
	uri := PathToURI(filepath.Join(root, "large.prompt.md"))
	var text strings.Builder
	for i := 0; i < 5000; i++ {
		text.WriteString("- inspect @missing/path and [Section]\n")
	}
	text.WriteString("# Section\n")
	s.Documents.Open(uri, text.String())
	s.SemanticCached(uri)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.SemanticCached(uri)
	}
}

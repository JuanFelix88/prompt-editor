package lang

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func referenceAt(line string, character int, fileFirst bool) *Reference {
	refs := ScanReferences(line)
	for pass := 0; pass < 2; pass++ {
		for i := range refs {
			r := &refs[i]
			if (pass == 0) == (r.File == fileFirst) && character >= r.StartUTF16 && character <= r.EndUTF16 {
				return r
			}
		}
	}
	return nil
}

func (s *Service) Hover(uri string, pos Position) *Hover {
	if !s.promptURI(uri) {
		return nil
	}
	text := s.Documents.Text(uri)
	lines := splitLines(text)
	if pos.Line < 0 || pos.Line >= len(lines) {
		return nil
	}
	r := referenceAt(lines[pos.Line], pos.Character, true)
	if r == nil {
		return nil
	}
	rng := Range{Start: Position{pos.Line, r.StartUTF16}, End: Position{pos.Line, r.EndUTF16}}
	if r.File {
		path, info, ok := Resolve(s.baseDir(uri), r.Value)
		if !ok {
			return &Hover{Contents: MarkupContent{Kind: "markdown", Value: fmt.Sprintf("`@%s`\n\nMissing file or folder relative to this document.", r.Value)}, Range: rng}
		}
		kind := "file"
		if info.IsDir() {
			kind = "folder"
		}
		return &Hover{Contents: MarkupContent{Kind: "markdown", Value: fmt.Sprintf("`@%s` — **%s**\n\n`%s`", r.Value, kind, path)}, Range: rng}
	}
	query := normalizeHeadingPath(r.Value)
	var found []Heading
	for _, h := range ScanHeadings(text) {
		if h.Path == query {
			found = append(found, h)
		}
	}
	if len(found) == 0 {
		return &Hover{Contents: MarkupContent{Kind: "markdown", Value: fmt.Sprintf("`[%s]`\n\nMissing heading in this document.", r.Value)}, Range: rng}
	}
	value := fmt.Sprintf("`[%s]` → line %d", r.Value, found[0].Line+1)
	if len(found) > 1 {
		value += fmt.Sprintf("\n\n%d matching headings; navigation uses the first.", len(found))
	}
	return &Hover{Contents: MarkupContent{Kind: "markdown", Value: value}, Range: rng}
}

// FileReferenceTarget resolves an explicit editor navigation request without
// requiring the target to exist. Unlike LSP definitions, this is available in
// any document; automatic Prompt Lang features retain their activation rules.
func (s *Service) FileReferenceTarget(uri string, pos Position) (string, bool) {
	lines := splitLines(s.Documents.Text(uri))
	if pos.Line < 0 || pos.Line >= len(lines) {
		return "", false
	}
	r := referenceAt(lines[pos.Line], pos.Character, true)
	if r == nil || !r.File || strings.HasSuffix(r.Value, "/") {
		return "", false
	}
	return referencePath(s.baseDir(uri), r.Value)
}
func (s *Service) Definition(uri string, pos Position) *Location {
	if !s.promptURI(uri) {
		return nil
	}
	text := s.Documents.Text(uri)
	lines := splitLines(text)
	if pos.Line < 0 || pos.Line >= len(lines) {
		return nil
	}
	r := referenceAt(lines[pos.Line], pos.Character, true)
	if r == nil {
		return nil
	}
	if r.File {
		path, _, ok := Resolve(s.baseDir(uri), r.Value)
		if !ok {
			return nil
		}
		return &Location{URI: PathToURI(path), Range: Range{}}
	}
	query := normalizeHeadingPath(r.Value)
	for _, h := range ScanHeadings(text) {
		if h.Path == query {
			return &Location{URI: uri, Range: Range{Start: Position{h.Line, 0}, End: Position{h.Line, h.LineUTF16}}}
		}
	}
	return nil
}

func headingTokenKind(level int) TokenKind { return TokenKind(int(TokenHeading1) + level - 1) }

func addSplit(tokens *[]SemanticToken, line, start, end int, kind TokenKind, refs []Reference) {
	cursor := start
	for _, r := range refs {
		if r.EndUTF16 <= start || r.StartUTF16 >= end {
			continue
		}
		a := r.StartUTF16
		if a < start {
			a = start
		}
		b := r.EndUTF16
		if b > end {
			b = end
		}
		if cursor < a {
			*tokens = append(*tokens, SemanticToken{Line: line, Start: cursor, Length: a - cursor, Kind: kind})
		}
		if b > cursor {
			cursor = b
		}
	}
	if cursor < end {
		*tokens = append(*tokens, SemanticToken{Line: line, Start: cursor, Length: end - cursor, Kind: kind})
	}
}

func (s *Service) referenceExists(base, value string, cached bool) bool {
	if !cached {
		_, _, ok := Resolve(base, value)
		return ok
	}
	key, ok := referencePath(base, value)
	if !ok {
		return false
	}
	now := time.Now()
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	if len(s.statuses) > 4096 {
		for key, status := range s.statuses {
			if now.Sub(status.checked) >= IndexTTL {
				delete(s.statuses, key)
			}
		}
		if len(s.statuses) > 8192 {
			s.statuses = make(map[string]cachedStatus)
		}
	}
	if status, ok := s.statuses[key]; ok && now.Sub(status.checked) < IndexTTL {
		return status.exists
	}
	_, _, ok = Resolve(base, value)
	s.statuses[key] = cachedStatus{exists: ok, checked: now}
	return ok
}

func (s *Service) Semantic(uri string) []SemanticToken       { return s.semantic(uri, false) }
func (s *Service) SemanticCached(uri string) []SemanticToken { return s.semantic(uri, true) }
func (s *Service) semantic(uri string, cachedStatus bool) []SemanticToken {
	text := s.Documents.Text(uri)
	lines := splitLines(text)
	prompt := s.promptURI(uri)
	base := s.baseDir(uri)
	headings := ScanHeadings(text)
	headingByLine := make(map[int]Heading, len(headings))
	headingPaths := make(map[string]bool, len(headings))
	for _, h := range headings {
		headingByLine[h.Line] = h
		headingPaths[h.Path] = true
	}
	var tokens []SemanticToken
	for n, line := range lines {
		var refs []Reference
		if prompt {
			refs = ScanReferences(line)
			sort.Slice(refs, func(i, j int) bool { return refs[i].StartUTF16 < refs[j].StartUTF16 })
		}
		if h, ok := headingByLine[n]; ok {
			tokens = append(tokens, SemanticToken{Line: n, Start: h.MarkerUTF16, Length: h.Level, Kind: TokenHeadingMarker})
			addSplit(&tokens, n, h.ContentStartUTF16, h.ContentEndUTF16, headingTokenKind(h.Level), refs)
		}
		if prompt {
			rs := []rune(line)
			i := 0
			for i < len(rs) && (rs[i] == ' ' || rs[i] == '\t') {
				i++
			}
			if i < len(rs) && (rs[i] == '-' || rs[i] == '*') && i+1 < len(rs) && (rs[i+1] == ' ' || rs[i+1] == '\t') {
				kind := TokenPromptItem
				if rs[i] == '*' {
					kind = TokenPromptStarItem
				}
				addSplit(&tokens, n, runeToUTF16(rs, i), UTF16Len(line), kind, refs)
			}
			for _, r := range refs {
				kind := TokenHeadingRefMissing
				if r.File {
					ok := s.referenceExists(base, r.Value, cachedStatus)
					kind = TokenFileMissing
					if ok {
						kind = TokenFile
					}
				} else if headingPaths[normalizeHeadingPath(r.Value)] {
					kind = TokenHeadingRef
				}
				tokens = append(tokens, SemanticToken{Line: n, Start: r.StartUTF16, Length: r.EndUTF16 - r.StartUTF16, Kind: kind})
			}
		}
	}
	sort.Slice(tokens, func(i, j int) bool {
		if tokens[i].Line != tokens[j].Line {
			return tokens[i].Line < tokens[j].Line
		}
		return tokens[i].Start < tokens[j].Start
	})
	// Broad spans were split around references, so sorted tokens are non-overlapping.
	return tokens
}

func EncodeSemantic(tokens []SemanticToken) []int {
	data := make([]int, 0, len(tokens)*5)
	prevLine, prevStart := 0, 0
	for _, t := range tokens {
		dl := t.Line - prevLine
		ds := t.Start
		if dl == 0 {
			ds = t.Start - prevStart
		}
		data = append(data, dl, ds, t.Length, int(t.Kind), 0)
		prevLine, prevStart = t.Line, t.Start
	}
	return data
}

func PlainTokenName(k TokenKind) string { return SemanticLegend[int(k)] }

func HeadingPaths(text string) []string {
	hs := ScanHeadings(text)
	out := make([]string, len(hs))
	for i, h := range hs {
		out[i] = h.Path
	}
	return out
}

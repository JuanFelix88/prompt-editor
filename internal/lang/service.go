package lang

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type cachedStatus struct {
	exists  bool
	checked time.Time
}
type Service struct {
	Root      string
	Documents *Documents
	Index     *WorkspaceIndex
	statusMu  sync.Mutex
	statuses  map[string]cachedStatus
}

func NewService(root string) (*Service, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &Service{Root: filepath.Clean(abs), Documents: NewDocuments(), Index: NewWorkspaceIndex(filepath.Clean(abs)), statuses: make(map[string]cachedStatus)}, nil
}

func (s *Service) promptURI(uri string) bool {
	path, err := URIToPath(uri)
	return err == nil && isPromptPath(path)
}

func (s *Service) Complete(uri string, pos Position) CompletionList {
	if !s.promptURI(uri) {
		return CompletionList{Items: []Completion{}}
	}
	text := s.Documents.Text(uri)
	lines := splitLines(text)
	if pos.Line < 0 || pos.Line >= len(lines) {
		return CompletionList{Items: []Completion{}}
	}
	line := lines[pos.Line]
	cursorByte := utf16ToByte(line, pos.Character)
	before := line[:cursorByte]
	if open := strings.LastIndex(before, "["); open >= 0 && !strings.Contains(before[open+1:], "]") &&
		(open == 0 || before[open-1] != '!') && !strings.Contains(before[open+1:], "[") {
		return headingCompletions(text, line, pos, open, cursorByte)
	}
	at := strings.LastIndex(before, "@")
	if at < 0 || at > 0 && isASCIIWord(rune(before[at-1])) {
		return CompletionList{Items: []Completion{}}
	}
	prefix := before[at+1:]
	for _, r := range prefix {
		if !isPathRune(r) {
			return CompletionList{Items: []Completion{}}
		}
	}
	return s.fileCompletions(prefix, pos, UTF16Len(before[:at+1]))
}

type scoredFile struct {
	e     IndexEntry
	score int
}

func (s *Service) fileCompletions(prefix string, pos Position, start int) CompletionList {
	entries, truncated := s.Index.Get()
	lower := strings.ToLower(prefix)
	matches := make([]scoredFile, 0)
	for _, e := range entries {
		path, base := strings.ToLower(e.Insert), strings.ToLower(e.Base)
		score := -1
		switch {
		case strings.HasPrefix(path, lower):
			score = 0
		case !strings.Contains(lower, "/") && strings.HasPrefix(base, lower):
			score = 1
		case strings.Contains(path, "/"+lower):
			score = 2
		case strings.Contains(path, lower):
			score = 3
		}
		if lower == "" {
			score = e.Depth
		}
		if score >= 0 {
			matches = append(matches, scoredFile{e, score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		a, b := matches[i], matches[j]
		if a.score != b.score {
			return a.score < b.score
		}
		if a.e.Directory != b.e.Directory {
			return a.e.Directory
		}
		if a.e.Depth != b.e.Depth {
			return a.e.Depth < b.e.Depth
		}
		return a.e.Insert < b.e.Insert
	})
	incomplete := truncated || len(matches) > MaxCompletions
	if len(matches) > MaxCompletions {
		matches = matches[:MaxCompletions]
	}
	items := make([]Completion, 0, len(matches))
	for _, m := range matches {
		parts := strings.Split(strings.TrimSuffix(m.e.Insert, "/"), "/")
		label := m.e.Insert
		if len(parts) > 2 {
			label = "../" + strings.Join(parts[len(parts)-2:], "/")
			if m.e.Directory {
				label += "/"
			}
		}
		kind, detail, group := 17, "file", "1"
		if m.e.Directory {
			kind, detail, group = 19, "folder", "0"
		}
		items = append(items, Completion{Label: label, Kind: kind, Detail: detail,
			SortText: fmt.Sprintf("%s_%05d_%s", group, m.e.Depth, m.e.Insert),
			TextEdit: TextEdit{Range: Range{Start: Position{pos.Line, start}, End: pos}, NewText: m.e.Insert}})
	}
	return CompletionList{IsIncomplete: incomplete, Items: items}
}

type scoredHeading struct {
	h     Heading
	score int
}

func headingCompletions(text, line string, pos Position, open, cursorByte int) CompletionList {
	prefix := normalizeHeadingPath(line[open+1 : cursorByte])
	lower := strings.ToLower(prefix)
	var matches []scoredHeading
	for _, h := range ScanHeadings(text) {
		path, leaf := strings.ToLower(h.Path), strings.ToLower(h.Text)
		score := -1
		switch {
		case lower == "":
			score = h.Level
		case strings.HasPrefix(path, lower):
			score = 0
		case strings.HasPrefix(leaf, lower):
			score = 1
		case strings.Contains(path, lower):
			score = 2
		}
		if score >= 0 {
			matches = append(matches, scoredHeading{h, score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		a, b := matches[i], matches[j]
		if a.score != b.score {
			return a.score < b.score
		}
		if a.h.Line != b.h.Line {
			return a.h.Line < b.h.Line
		}
		return a.h.Path < b.h.Path
	})
	incomplete := len(matches) > MaxCompletions
	if incomplete {
		matches = matches[:MaxCompletions]
	}
	closing := cursorByte < len(line) && line[cursorByte] == ']'
	items := make([]Completion, 0, len(matches))
	start := UTF16Len(line[:open+1])
	for i, m := range matches {
		insert := m.h.Path
		if !closing {
			insert += "]"
		}
		items = append(items, Completion{Label: m.h.Path, Kind: 18, Detail: fmt.Sprintf("line %d", m.h.Line+1), FilterText: m.h.Path,
			SortText: fmt.Sprintf("%05d", i), TextEdit: TextEdit{Range: Range{Start: Position{pos.Line, start}, End: pos}, NewText: insert}})
	}
	return CompletionList{IsIncomplete: incomplete, Items: items}
}

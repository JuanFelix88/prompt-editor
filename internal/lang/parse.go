package lang

import (
	"strings"
	"unicode"
)

func ScanHeadings(text string) []Heading {
	lines := splitLines(text)
	var out []Heading
	stack := make([]string, 6)
	inFence := false
	var fence rune
	fenceLen := 0

	for lineNo, line := range lines {
		rs := []rune(line)
		i := 0
		for i < len(rs) && i < 4 && rs[i] == ' ' {
			i++
		}
		if i <= 3 && i < len(rs) && (rs[i] == '`' || rs[i] == '~') {
			mark := rs[i]
			j := i
			for j < len(rs) && rs[j] == mark {
				j++
			}
			if j-i >= 3 {
				if !inFence {
					inFence, fence, fenceLen = true, mark, j-i
					continue
				}
				if mark == fence && j-i >= fenceLen {
					inFence = false
					continue
				}
			}
		}
		if inFence || i > 3 || i >= len(rs) || rs[i] != '#' {
			continue
		}
		markerStart := i
		for i < len(rs) && rs[i] == '#' {
			i++
		}
		level := i - markerStart
		if level < 1 || level > 6 || (i < len(rs) && rs[i] != ' ' && rs[i] != '\t') {
			continue
		}
		contentStart := i
		for contentStart < len(rs) && (rs[contentStart] == ' ' || rs[contentStart] == '\t') {
			contentStart++
		}
		end := len(rs)
		for end > contentStart && unicode.IsSpace(rs[end-1]) {
			end--
		}
		closeStart := end
		for closeStart > contentStart && rs[closeStart-1] == '#' {
			closeStart--
		}
		if closeStart < end && closeStart > contentStart && unicode.IsSpace(rs[closeStart-1]) {
			end = closeStart - 1
			for end > contentStart && unicode.IsSpace(rs[end-1]) {
				end--
			}
		}
		if end <= contentStart {
			continue
		}
		text := strings.TrimSpace(string(rs[contentStart:end]))
		if text == "" {
			continue
		}
		for n := level - 1; n < len(stack); n++ {
			stack[n] = ""
		}
		stack[level-1] = text
		parts := make([]string, 0, level)
		for _, part := range stack[:level] {
			if part != "" {
				parts = append(parts, part)
			}
		}
		out = append(out, Heading{
			Level: level, Line: lineNo, LineUTF16: UTF16Len(line),
			MarkerUTF16:       runeToUTF16(rs, markerStart),
			ContentStartUTF16: runeToUTF16(rs, contentStart), ContentEndUTF16: runeToUTF16(rs, end),
			Text: text, Path: strings.Join(parts, " > "),
		})
	}
	return out
}

func normalizeHeadingPath(s string) string {
	parts := strings.Split(s, ">")
	out := parts[:0]
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, " > ")
}

func isASCIIWord(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_'
}
func isPathRune(r rune) bool {
	return isASCIIWord(r) || r == '.' || r == '~' || r == '+' || r == '/' || r == '-'
}

// ScanTaskMarker reports the inner character of a task marker opening a line,
// as in `- [x] `, `* [ ] `, or `+ [*] `. Recognition is deliberately
// positional: a bracketed value elsewhere on the line is ordinary text, so
// `see [x]` never produces a task marker.
func ScanTaskMarker(line string) (TaskMarker, bool) { return scanTaskMarker([]rune(line)) }

// ScanTaskMarkerRunes is ScanTaskMarker over an already-decoded line, so
// render paths reuse the line buffer instead of re-allocating every frame.
func ScanTaskMarkerRunes(rs []rune) (TaskMarker, bool) { return scanTaskMarker(rs) }

func scanTaskMarker(rs []rune) (TaskMarker, bool) {
	i := 0
	for i < len(rs) && (rs[i] == ' ' || rs[i] == '\t') {
		i++
	}
	if i+1 >= len(rs) || rs[i] != '-' && rs[i] != '*' && rs[i] != '+' {
		return TaskMarker{}, false
	}
	if rs[i+1] != ' ' && rs[i+1] != '\t' {
		return TaskMarker{}, false
	}
	j := i + 2
	for j < len(rs) && (rs[j] == ' ' || rs[j] == '\t') {
		j++
	}
	if j+2 >= len(rs) || rs[j] != '[' || rs[j+2] != ']' {
		return TaskMarker{}, false
	}
	inner := rs[j+1]
	if inner == ' ' || inner == '\t' || inner == '[' || inner == ']' {
		return TaskMarker{}, false
	}
	if j+3 < len(rs) && rs[j+3] != ' ' && rs[j+3] != '\t' {
		return TaskMarker{}, false
	}
	return TaskMarker{
		StartRune: j + 1, EndRune: j + 2,
		StartUTF16: runeToUTF16(rs, j+1), EndUTF16: runeToUTF16(rs, j+2),
		Value: string(inner),
	}, true
}

// TaskMarkerKind maps a marker value to its token, separating a completed
// task from any other custom marker.
func TaskMarkerKind(value string) TokenKind {
	if strings.EqualFold(value, "x") {
		return TokenTaskDone
	}
	return TokenTaskMarker
}
func ScanReferences(line string) []Reference {
	rs := []rune(line)
	taskAt := -1
	if t, ok := ScanTaskMarker(line); ok {
		taskAt = t.StartRune - 1
	}
	files := make([]Reference, 0)
	for i := 0; i < len(rs); i++ {
		if rs[i] != '@' || (i > 0 && isASCIIWord(rs[i-1])) {
			continue
		}
		j := i + 1
		for j < len(rs) && isPathRune(rs[j]) {
			j++
		}
		if j == i+1 {
			continue
		}
		files = append(files, Reference{StartRune: i, EndRune: j, StartUTF16: runeToUTF16(rs, i), EndUTF16: runeToUTF16(rs, j), Value: string(rs[i+1 : j]), File: true})
		i = j - 1
	}
	out := append([]Reference(nil), files...)
	for i := 0; i < len(rs); i++ {
		if rs[i] != '[' || (i > 0 && rs[i-1] == '!') || i == taskAt {
			continue
		}
		j := i + 1
		for j < len(rs) && rs[j] != ']' {
			j++
		}
		if j >= len(rs) {
			continue
		}
		end := j + 1
		value := strings.TrimSpace(string(rs[i+1 : j]))
		if value == "" || strings.EqualFold(value, "x") && len([]rune(value)) == 1 || (end < len(rs) && rs[end] == '(') {
			i = j
			continue
		}
		overlaps := false
		for _, f := range files {
			if i < f.EndRune && end > f.StartRune {
				overlaps = true
				break
			}
		}
		if !overlaps {
			out = append(out, Reference{StartRune: i, EndRune: end, StartUTF16: runeToUTF16(rs, i), EndUTF16: runeToUTF16(rs, end), Value: value})
		}
		i = j
	}
	return out
}

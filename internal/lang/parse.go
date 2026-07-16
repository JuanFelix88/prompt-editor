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

func ScanReferences(line string) []Reference {
	rs := []rune(line)
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
		if rs[i] != '[' || (i > 0 && rs[i-1] == '!') {
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

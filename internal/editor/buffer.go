package editor

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"

	"github.com/juanandrade/prompt-cli/internal/lang"
)

type Buffer struct {
	Path     string
	Lines    [][]rune
	Row, Col int
	Dirty    bool
	Version  uint64
}

func Load(path string) (*Buffer, error) {
	b := &Buffer{Path: path, Lines: [][]rune{{}}}
	info, err := os.Stat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil && !info.Mode().IsRegular() {
		return nil, errors.New("only regular text files can be edited")
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			return nil, errors.New("file is not UTF-8 text")
		}
		text := strings.ReplaceAll(string(data), "\r\n", "\n")
		parts := strings.Split(text, "\n")
		b.Lines = make([][]rune, len(parts))
		for i, p := range parts {
			b.Lines[i] = []rune(p)
		}
	}
	return b, nil
}
func (b *Buffer) Text() string {
	parts := make([]string, len(b.Lines))
	for i, l := range b.Lines {
		parts[i] = string(l)
	}
	return strings.Join(parts, "\n")
}
func (b *Buffer) touch() { b.Dirty = true; b.Version++ }
func (b *Buffer) InsertRune(r rune) {
	line := b.Lines[b.Row]
	line = append(line, 0)
	copy(line[b.Col+1:], line[b.Col:])
	line[b.Col] = r
	b.Lines[b.Row] = line
	b.Col++
	b.touch()
}
func (b *Buffer) CurrentRune() (rune, bool) {
	line := b.Lines[b.Row]
	if b.Col >= len(line) {
		return 0, false
	}
	return line[b.Col], true
}
func (b *Buffer) InsertPair(open, close rune) { b.InsertRune(open); b.InsertRune(close); b.Left() }
func (b *Buffer) EmptyPairAtCaret() bool {
	if b.Col == 0 || b.Col >= len(b.Lines[b.Row]) {
		return false
	}
	left, right := b.Lines[b.Row][b.Col-1], b.Lines[b.Row][b.Col]
	return left == '(' && right == ')' || left == '[' && right == ']' || left == '{' && right == '}' || left == '"' && right == '"' || left == '\'' && right == '\'' || left == '`' && right == '`'
}

func (b *Buffer) InsertText(s string) {
	for _, r := range s {
		if r == '\n' {
			b.Newline()
		} else if r == '\t' || r >= 32 && r != 127 {
			b.InsertRune(r)
		}
	}
}
func lineIndent(line []rune) []rune {
	end := 0
	for end < len(line) && (line[end] == ' ' || line[end] == '\t') {
		end++
	}
	return append([]rune(nil), line[:end]...)
}
func (b *Buffer) insertLine(at int, line []rune) {
	b.Lines = append(b.Lines, nil)
	copy(b.Lines[at+1:], b.Lines[at:])
	b.Lines[at] = line
	b.Row, b.Col = at, len(line)
	b.touch()
}
func (b *Buffer) InsertLineBelow() { b.insertLine(b.Row+1, lineIndent(b.Lines[b.Row])) }
func (b *Buffer) InsertLineAbove() { b.insertLine(b.Row, lineIndent(b.Lines[b.Row])) }
func (b *Buffer) MoveLineUp() {
	if b.Row == 0 {
		return
	}
	b.Lines[b.Row-1], b.Lines[b.Row] = b.Lines[b.Row], b.Lines[b.Row-1]
	b.Row--
	b.touch()
}
func (b *Buffer) MoveLineDown() {
	if b.Row+1 >= len(b.Lines) {
		return
	}
	b.Lines[b.Row], b.Lines[b.Row+1] = b.Lines[b.Row+1], b.Lines[b.Row]
	b.Row++
	b.touch()
}

type wordClass uint8

const (
	wordSpace wordClass = iota
	wordText
	wordOther
)

func classifyGrapheme(cluster []rune) wordClass {
	allSpace := len(cluster) > 0
	for _, r := range cluster {
		if !unicode.IsSpace(r) {
			allSpace = false
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return wordText
		}
	}
	if allSpace {
		return wordSpace
	}
	return wordOther
}
func graphemeIndex(bounds []int, col int) int {
	for i, bound := range bounds {
		if bound >= col {
			return i
		}
	}
	return len(bounds) - 1
}
func wordStart(line []rune, col int) int {
	bounds := graphemeBounds(line)
	i := graphemeIndex(bounds, col)
	for i > 0 && classifyGrapheme(line[bounds[i-1]:bounds[i]]) == wordSpace {
		i--
	}
	if i > 0 {
		class := classifyGrapheme(line[bounds[i-1]:bounds[i]])
		for i > 0 && classifyGrapheme(line[bounds[i-1]:bounds[i]]) == class {
			i--
		}
	}
	return bounds[i]
}
func wordEnd(line []rune, col int, includeTrailingSpace bool) int {
	bounds := graphemeBounds(line)
	i := graphemeIndex(bounds, col)
	if i >= len(bounds)-1 {
		return len(line)
	}
	if classifyGrapheme(line[bounds[i]:bounds[i+1]]) == wordSpace {
		for i < len(bounds)-1 && classifyGrapheme(line[bounds[i]:bounds[i+1]]) == wordSpace {
			i++
		}
		if !includeTrailingSpace && i < len(bounds)-1 {
			class := classifyGrapheme(line[bounds[i]:bounds[i+1]])
			for i < len(bounds)-1 && classifyGrapheme(line[bounds[i]:bounds[i+1]]) == class {
				i++
			}
		}
		return bounds[i]
	}
	class := classifyGrapheme(line[bounds[i]:bounds[i+1]])
	for i < len(bounds)-1 && classifyGrapheme(line[bounds[i]:bounds[i+1]]) == class {
		i++
	}
	if includeTrailingSpace {
		for i < len(bounds)-1 && classifyGrapheme(line[bounds[i]:bounds[i+1]]) == wordSpace {
			i++
		}
	}
	return bounds[i]
}
func (b *Buffer) DeleteWordBackward() {
	if b.Col == 0 {
		b.Backspace()
		return
	}
	line, end := b.Lines[b.Row], b.Col
	start := wordStart(line, end)
	b.Lines[b.Row] = append(line[:start], line[end:]...)
	b.Col = start
	b.touch()
}
func (b *Buffer) DeleteWordForward() {
	line := b.Lines[b.Row]
	if b.Col == len(line) {
		b.Delete()
		return
	}
	end := wordEnd(line, b.Col, false)
	b.Lines[b.Row] = append(line[:b.Col], line[end:]...)
	b.touch()
}
func (b *Buffer) WordLeft() {
	if b.Col == 0 {
		b.Left()
		return
	}
	b.Col = wordStart(b.Lines[b.Row], b.Col)
}
func (b *Buffer) WordRight() {
	if b.Col == len(b.Lines[b.Row]) {
		b.Right()
		return
	}
	b.Col = wordEnd(b.Lines[b.Row], b.Col, true)
}

func (b *Buffer) Newline() {
	line := b.Lines[b.Row]
	tail := append([]rune(nil), line[b.Col:]...)
	b.Lines[b.Row] = line[:b.Col]
	b.Lines = append(b.Lines, nil)
	copy(b.Lines[b.Row+2:], b.Lines[b.Row+1:])
	b.Lines[b.Row+1] = tail
	b.Row++
	b.Col = 0
	b.touch()
}
func graphemeBounds(rs []rune) []int {
	g := uniseg.NewGraphemes(string(rs))
	out := []int{0}
	n := 0
	for g.Next() {
		n += len([]rune(g.Str()))
		out = append(out, n)
	}
	return out
}
func previousBound(rs []rune, col int) int {
	prev := 0
	for _, n := range graphemeBounds(rs) {
		if n >= col {
			return prev
		}
		prev = n
	}
	return prev
}
func nextBound(rs []rune, col int) int {
	for _, n := range graphemeBounds(rs) {
		if n > col {
			return n
		}
	}
	return len(rs)
}
func (b *Buffer) Backspace() {
	if b.Col > 0 {
		line := b.Lines[b.Row]
		p := previousBound(line, b.Col)
		b.Lines[b.Row] = append(line[:p], line[b.Col:]...)
		b.Col = p
		b.touch()
		return
	}
	if b.Row > 0 {
		prev := len(b.Lines[b.Row-1])
		b.Lines[b.Row-1] = append(b.Lines[b.Row-1], b.Lines[b.Row]...)
		b.Lines = append(b.Lines[:b.Row], b.Lines[b.Row+1:]...)
		b.Row--
		b.Col = prev
		b.touch()
	}
}
func (b *Buffer) Delete() {
	line := b.Lines[b.Row]
	if b.Col < len(line) {
		n := nextBound(line, b.Col)
		b.Lines[b.Row] = append(line[:b.Col], line[n:]...)
		b.touch()
		return
	}
	if b.Row+1 < len(b.Lines) {
		b.Lines[b.Row] = append(line, b.Lines[b.Row+1]...)
		b.Lines = append(b.Lines[:b.Row+1], b.Lines[b.Row+2:]...)
		b.touch()
	}
}
func (b *Buffer) Left() {
	if b.Col > 0 {
		b.Col = previousBound(b.Lines[b.Row], b.Col)
	} else if b.Row > 0 {
		b.Row--
		b.Col = len(b.Lines[b.Row])
	}
}
func (b *Buffer) Right() {
	if b.Col < len(b.Lines[b.Row]) {
		b.Col = nextBound(b.Lines[b.Row], b.Col)
	} else if b.Row+1 < len(b.Lines) {
		b.Row++
		b.Col = 0
	}
}
func (b *Buffer) Up() {
	if b.Row > 0 {
		b.Row--
		if b.Col > len(b.Lines[b.Row]) {
			b.Col = len(b.Lines[b.Row])
		}
		b.Col = previousOrEqual(b.Lines[b.Row], b.Col)
	}
}
func (b *Buffer) Down() {
	if b.Row+1 < len(b.Lines) {
		b.Row++
		if b.Col > len(b.Lines[b.Row]) {
			b.Col = len(b.Lines[b.Row])
		}
		b.Col = previousOrEqual(b.Lines[b.Row], b.Col)
	}
}
func previousOrEqual(rs []rune, col int) int {
	last := 0
	for _, n := range graphemeBounds(rs) {
		if n > col {
			return last
		}
		last = n
	}
	return last
}

func utf16RuneIndex(rs []rune, units int) int {
	u := 0
	for i, r := range rs {
		w := 1
		if r > 0xffff {
			w = 2
		}
		if u+w > units {
			return i
		}
		u += w
		if u == units {
			return i + 1
		}
	}
	return len(rs)
}
func (b *Buffer) ApplyCompletion(c lang.Completion) {
	if c.TextEdit.Range.Start.Line != b.Row || c.TextEdit.Range.End.Line != b.Row {
		return
	}
	line := b.Lines[b.Row]
	start := utf16RuneIndex(line, c.TextEdit.Range.Start.Character)
	end := utf16RuneIndex(line, c.TextEdit.Range.End.Character)
	insert := []rune(c.TextEdit.NewText)
	next := make([]rune, 0, len(line)-end+start+len(insert))
	next = append(next, line[:start]...)
	next = append(next, insert...)
	next = append(next, line[end:]...)
	b.Lines[b.Row] = next
	b.Col = start + len(insert)
	b.touch()
}

func (b *Buffer) Save() error {
	dir := filepath.Dir(b.Path)
	tmp, err := os.CreateTemp(dir, ".prompt-cli-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	ok := false
	defer func() {
		tmp.Close()
		if !ok {
			os.Remove(name)
		}
	}()
	if info, e := os.Stat(b.Path); e == nil {
		_ = tmp.Chmod(info.Mode())
	}
	if _, err = tmp.WriteString(b.Text()); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, b.Path)
	}
	if err != nil {
		return err
	}
	ok = true
	b.Dirty = false
	return nil
}

func (b *Buffer) SetPosition(line, column int) {
	if line < 1 {
		line = 1
	}
	if line > len(b.Lines) {
		line = len(b.Lines)
	}
	b.Row = line - 1
	if column < 1 {
		column = 1
	}
	b.Col = column - 1
	if b.Col > len(b.Lines[b.Row]) {
		b.Col = len(b.Lines[b.Row])
	}
	b.Col = previousOrEqual(b.Lines[b.Row], b.Col)
}

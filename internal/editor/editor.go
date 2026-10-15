package editor

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
	"golang.org/x/term"

	"github.com/juanandrade/prompt-cli/internal/lang"
)

var ErrCancelled = errors.New("editing cancelled")
var autoPairs = map[rune]rune{'(': ')', '[': ']', '{': '}', '"': '"', '\'': '\'', '`': '`'}

const enableKeyboardEnhancements = "\x1b[>4;2m\x1b[>1u"
const disableKeyboardEnhancements = "\x1b[<u\x1b[>4;0m"

func (e *Editor) terminalTitle() string {
	name := "Untitled"
	if !e.anonymous {
		name = filepath.Base(e.buffer.Path)
	}
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	return "\x1b]0;prompt-cli — " + name + "\x07"
}

type Editor struct {
	buffer                             *Buffer
	service                            *lang.Service
	uri                                string
	top, hscroll                       int
	rows, cols                         int
	completions                        []lang.Completion
	selected                           int
	tokens                             []lang.SemanticToken
	structureVersion                   uint64
	insideFence, delimiter, metadata   []bool
	status                             string
	gotoInput, gotoError               string
	openedAt                           time.Time
	anonymous, confirmCancel, gotoOpen bool
	keepOpen                           bool
	plainText                          bool
	output                             *os.File
}

func New(path, workspace string, line, column int) (*Editor, error) {
	lower := strings.ToLower(path)
	if !strings.HasSuffix(lower, ".md") && !strings.HasSuffix(lower, ".prompt") {
		return nil, errors.New("only Markdown (.md) and Prompt Lang (.prompt) files can be edited")
	}
	b, err := Load(path)
	if err != nil {
		return nil, err
	}
	b.SetPosition(line, column)
	return newEditor(b, lang.PathToURI(path), workspace, false)
}

func NewUntitled(workspace string, line, column int) (*Editor, error) {
	b := &Buffer{Lines: [][]rune{{}}}
	b.SetPosition(line, column)
	return newEditor(b, lang.PathToURI(filepath.Join(workspace, "prompt.md")), workspace, true)
}

func newEditor(b *Buffer, uri, workspace string, anonymous bool) (*Editor, error) {
	s, err := lang.NewService(workspace)
	if err != nil {
		return nil, err
	}
	s.Documents.Open(uri, b.Text())
	e := &Editor{buffer: b, service: s, uri: uri, anonymous: anonymous, status: "Ctrl+S save/confirm • Ctrl+Shift+S save • Ctrl+O/F12 open • Ctrl+X cancel"}
	ext := strings.ToLower(filepath.Ext(b.Path))
	e.plainText = !anonymous && ext != ".md" && ext != ".prompt"
	e.tokens = s.Semantic(uri)
	return e, nil
}

func (e *Editor) Run() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", errors.New("stdin is not an interactive terminal")
	}
	e.output = os.Stderr
	if !e.anonymous && !term.IsTerminal(int(os.Stderr.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
		e.output = os.Stdout
	}
	if !term.IsTerminal(int(e.output.Fd())) {
		return "", errors.New("editor output is not an interactive terminal")
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		return "", err
	}
	cleanup := func() {
		_, _ = io.WriteString(e.output, disableKeyboardEnhancements+"\x1b[?2004l\x1b[?1049l\x1b[?25h\x1b[0m")
		_ = term.Restore(fd, old)
	}
	defer cleanup()
	e.openedAt = time.Now()
	_, _ = io.WriteString(e.output, e.terminalTitle()+"\x1b[?1049h\x1b[2J\x1b[H"+enableKeyboardEnhancements+"\x1b[?2004h\x1b[?25h")

	done := make(chan struct{})
	defer close(done)
	resize := watchResize(done)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, terminationSignals()...)
	defer signal.Stop(interrupt)
	keys, inputErr := startInput(os.Stdin)
	clock := time.NewTicker(time.Second)
	defer clock.Stop()
	var timer *time.Timer
	var analysis <-chan time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	session := &editorSession{current: e}
	schedule := func() {
		if timer != nil {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		}
		timer = time.NewTimer(75 * time.Millisecond)
		analysis = timer.C
	}

	e.resize()
	if err = e.render(); err != nil {
		return "", err
	}
	for {
		select {
		case received := <-interrupt:
			if received == os.Interrupt {
				e.confirmCancel = true
				e.completions = nil
				if err = e.render(); err != nil {
					return "", err
				}
				continue
			}
			return "", ErrCancelled
		case inputError := <-inputErr:
			if errors.Is(inputError, io.EOF) {
				return "", ErrCancelled
			}
			return "", inputError
		case <-resize:
			oldRows, oldCols := e.rows, e.cols
			e.resize()
			if oldRows != e.rows || oldCols != e.cols {
				if err = e.render(); err != nil {
					return "", err
				}
			}
		case <-clock.C:
			if err = e.renderElapsed(); err != nil {
				return "", err
			}
		case <-analysis:
			e.service.Documents.Change(e.uri, e.buffer.Text())
			e.tokens = e.service.SemanticCached(e.uri)
			analysis = nil
			if err = e.render(); err != nil {
				return "", err
			}
		case key := <-keys:
			if key.Kind == KeySuspend {
				if err = suspendTerminal(fd, old, e.output); err != nil {
					return "", err
				}
				e.resize()
				if err = e.render(); err != nil {
					return "", err
				}
				continue
			}
			changed, complete, exit := session.handle(key)
			if exit == 1 {
				return session.result, nil
			}
			if exit == -1 {
				return "", ErrCancelled
			}
			if e != session.current {
				// Analysis timers belong to the buffer that scheduled them.
				if timer != nil {
					timer.Stop()
				}
				analysis = nil
				e = session.current
				e.resize()
				if _, err = io.WriteString(e.output, e.terminalTitle()); err != nil {
					return "", err
				}
			}
			if changed {
				schedule()
			}
			if complete {
				e.refreshCompletion()
			} else if changed && len(e.completions) > 0 {
				e.refreshCompletion()
			}
			if err = e.render(); err != nil {
				return "", err
			}
		}
	}
}

func (e *Editor) confirm() (string, bool) {
	if e.anonymous {
		return e.buffer.Text(), true
	}
	if err := e.buffer.Save(); err != nil {
		e.status = "save failed: " + err.Error()
		return "", false
	}
	return "", true
}
func (e *Editor) saveWithoutExit() {
	if e.anonymous {
		e.status = "untitled not written; disable Keep open (Ctrl+K), then Ctrl+S to confirm"
		return
	}
	if err := e.buffer.Save(); err != nil {
		e.status = "save failed: " + err.Error()
		return
	}
	e.status = "saved"
}

func parseGotoInput(input string) (line, column int, ok bool) {
	parts := strings.Split(input, ":")
	if len(parts) == 1 {
		line, err := strconv.Atoi(parts[0])
		return line, 1, err == nil
	}
	if len(parts) != 2 {
		return 0, 0, false
	}
	line, lineErr := strconv.Atoi(parts[0])
	column, columnErr := strconv.Atoi(parts[1])
	return line, column, lineErr == nil && columnErr == nil
}

func (e *Editor) handleGoto(k Key) {
	switch k.Kind {
	case KeyEscape, KeyCancel:
		e.gotoOpen, e.gotoError = false, ""
	case KeyEnter:
		line, column, ok := parseGotoInput(e.gotoInput)
		if !ok {
			e.gotoError = "formato inválido; use linha:coluna"
			return
		}
		e.buffer.SetPosition(line, column)
		e.gotoOpen, e.gotoError = false, ""
	case KeyBackspace:
		runes := []rune(e.gotoInput)
		if len(runes) > 0 {
			e.gotoInput = string(runes[:len(runes)-1])
		}
		e.gotoError = ""
	case KeyRune:
		if k.Rune >= '0' && k.Rune <= '9' || k.Rune == ':' {
			e.gotoInput += string(k.Rune)
			e.gotoError = ""
		}
	case KeyPaste:
		for _, r := range k.Text {
			if r >= '0' && r <= '9' || r == ':' {
				e.gotoInput += string(r)
			}
		}
		e.gotoError = ""
	}
}

func (e *Editor) handle(k Key) (changed, complete bool, exit int) {
	before := e.buffer.Version
	if e.gotoOpen {
		e.handleGoto(k)
		return false, false, 0
	}
	if e.confirmCancel {
		if k.Kind == KeyRune && (k.Rune == 'y' || k.Rune == 'Y') {
			return false, false, -1
		}
		if k.Kind == KeyRune && (k.Rune == 'n' || k.Rune == 'N') || k.Kind == KeyEscape || k.Kind == KeyCancel || k.Kind == KeyEnter {
			e.confirmCancel = false
		}
		return false, false, 0
	}
	switch k.Kind {
	case KeySave:
		if !e.keepOpen {
			return false, false, 1
		}
		e.completions = nil
		e.saveWithoutExit()
	case KeySaveWithoutExit:
		e.completions = nil
		e.saveWithoutExit()
	case KeyToggleKeepOpen:
		e.keepOpen = !e.keepOpen
		e.completions = nil
	case KeyCancel:
		e.confirmCancel = true
		e.completions = nil
		return false, false, 0
	case KeyEscape:
		e.completions = nil
	case KeyGoTo:
		e.completions = nil
		e.gotoInput, e.gotoError, e.gotoOpen = "", "", true
	case KeyComplete:
		complete = true
	case KeyTab:
		if len(e.completions) > 0 {
			e.accept()
		} else {
			e.buffer.InsertRune('\t')
		}
	case KeyEnter:
		if len(e.completions) > 0 {
			e.accept()
		} else {
			e.buffer.Newline()
		}
	case KeyRune:
		if current, ok := e.buffer.CurrentRune(); ok && current == k.Rune && strings.ContainsRune(")]}'\"`", k.Rune) {
			e.buffer.Right()
		} else if close, ok := autoPairs[k.Rune]; ok {
			e.buffer.InsertPair(k.Rune, close)
		} else {
			e.buffer.InsertRune(k.Rune)
		}
		complete = k.Rune == '@' || k.Rune == '[' || k.Rune == '/'
	case KeyPaste:
		e.buffer.InsertText(k.Text)
	case KeyBackspace:
		pair := e.buffer.EmptyPairAtCaret()
		e.buffer.Backspace()
		if pair {
			e.buffer.Delete()
		}
	case KeyDeleteWord:
		e.buffer.DeleteWordBackward()
	case KeyDeleteWordForward:
		e.buffer.DeleteWordForward()
	case KeyInsertLineBelow:
		e.completions = nil
		e.buffer.InsertLineBelow()
	case KeyInsertLineAbove:
		e.completions = nil
		e.buffer.InsertLineAbove()
	case KeyMoveLineUp:
		e.completions = nil
		e.buffer.MoveLineUp()
	case KeyMoveLineDown:
		e.completions = nil
		e.buffer.MoveLineDown()
	case KeyDelete:
		e.buffer.Delete()
	case KeyLeft:
		e.completions = nil
		e.buffer.Left()
	case KeyRight:
		e.completions = nil
		e.buffer.Right()
	case KeyWordLeft:
		e.completions = nil
		e.buffer.WordLeft()
	case KeyWordRight:
		e.completions = nil
		e.buffer.WordRight()
	case KeyHome:
		e.completions = nil
		e.buffer.Col = 0
	case KeyEnd:
		e.completions = nil
		e.buffer.Col = len(e.buffer.Lines[e.buffer.Row])
	case KeyDocumentHome:
		e.completions = nil
		e.buffer.Row, e.buffer.Col = 0, 0
	case KeyDocumentEnd:
		e.completions = nil
		e.buffer.Row = len(e.buffer.Lines) - 1
		e.buffer.Col = len(e.buffer.Lines[e.buffer.Row])
	case KeyUp:
		if len(e.completions) > 0 {
			e.selected--
			if e.selected < 0 {
				e.selected = len(e.completions) - 1
			}
		} else {
			e.buffer.Up()
		}
	case KeyDown:
		if len(e.completions) > 0 {
			e.selected = (e.selected + 1) % len(e.completions)
		} else {
			e.buffer.Down()
		}
	case KeyPageUp:
		e.completions = nil
		for i := 0; i < max(1, e.rows-2); i++ {
			e.buffer.Up()
		}
	case KeyPageDown:
		e.completions = nil
		for i := 0; i < max(1, e.rows-2); i++ {
			e.buffer.Down()
		}
	}
	return e.buffer.Version != before, complete, 0
}
func (e *Editor) accept() {
	if len(e.completions) == 0 {
		return
	}
	e.buffer.ApplyCompletion(e.completions[e.selected])
	e.completions = nil
}
func (e *Editor) refreshCompletion() {
	e.service.Documents.Change(e.uri, e.buffer.Text())
	pos := lang.Position{Line: e.buffer.Row, Character: len(utf16.Encode(e.buffer.Lines[e.buffer.Row][:e.buffer.Col]))}
	list := e.service.Complete(e.uri, pos)
	e.completions = list.Items
	e.selected = 0
}
func (e *Editor) resize() {
	cols, rows, err := term.GetSize(int(e.output.Fd()))
	if err != nil {
		cols, rows = 80, 24
	}
	e.cols, e.rows = max(1, cols), max(1, rows)
}

func digits(n int) int { return len(strconv.Itoa(max(1, n))) }
func visualWidth(rs []rune) int {
	col := 0
	g := uniseg.NewGraphemes(string(rs))
	for g.Next() {
		s := g.Str()
		if s == "\t" {
			col += 4 - col%4
		} else {
			_, w := safeDisplay(s)
			col += w
		}
	}
	return col
}
func (e *Editor) layout() (gutter, editorRows, caretRow, caretCol int) {
	gutter = digits(len(e.buffer.Lines)) + 2
	if e.cols <= gutter {
		gutter = 0
	}
	editorRows = e.rows
	if e.rows >= 2 {
		editorRows--
	}
	if e.buffer.Row < e.top {
		e.top = e.buffer.Row
	}
	if e.buffer.Row >= e.top+editorRows {
		e.top = e.buffer.Row - editorRows + 1
	}
	contentWidth := max(1, e.cols-gutter)
	x := visualWidth(e.buffer.Lines[e.buffer.Row][:e.buffer.Col])
	if x < e.hscroll {
		e.hscroll = x
	}
	if x >= e.hscroll+contentWidth {
		e.hscroll = x - contentWidth + 1
	}
	caretRow = e.buffer.Row - e.top
	caretCol = gutter + x - e.hscroll
	if caretCol >= e.cols {
		caretCol = e.cols - 1
	}
	if caretCol < 0 {
		caretCol = 0
	}
	if caretRow >= editorRows {
		caretRow = editorRows - 1
	}
	return
}

func tokenStyle(k lang.TokenKind) string {
	switch k {
	case lang.TokenFile:
		return "\x1b[38;5;75m"
	case lang.TokenFileMissing, lang.TokenHeadingRefMissing:
		return "\x1b[4;38;5;203m"
	case lang.TokenHeadingRef:
		return "\x1b[38;5;81m"
	case lang.TokenHeadingMarker:
		return "\x1b[38;5;244m"
	case lang.TokenHeading1, lang.TokenHeading4:
		return "\x1b[1;38;5;141m"
	case lang.TokenHeading2, lang.TokenHeading5:
		return "\x1b[38;5;81m"
	case lang.TokenHeading3, lang.TokenHeading6:
		return "\x1b[38;5;220m"
	case lang.TokenPromptItem:
		return "\x1b[31m"
	case lang.TokenPromptStarItem:
		return "\x1b[34m"
	}
	return "\x1b[0m"
}
func runeIndexUTF16(rs []rune, unit int) int {
	u := 0
	for i, r := range rs {
		w := 1
		if r > 0xffff {
			w = 2
		}
		if u+w > unit {
			return i
		}
		u += w
		if u == unit {
			return i + 1
		}
	}
	return len(rs)
}
func fillStyle(styles []string, start, end int, style string) {
	if start < 0 {
		start = 0
	}
	if end > len(styles) {
		end = len(styles)
	}
	for i := start; i < end; i++ {
		styles[i] = style
	}
}
func fenceMarker(rs []rune) (rune, int, bool) {
	i := 0
	for i < len(rs) && i < 4 && rs[i] == ' ' {
		i++
	}
	if i > 3 || i >= len(rs) || rs[i] != '`' && rs[i] != '~' {
		return 0, 0, false
	}
	mark, j := rs[i], i
	for j < len(rs) && rs[j] == mark {
		j++
	}
	return mark, j - i, j-i >= 3
}
func (e *Editor) structuralStates() (inside, delimiter, metadata []bool) {
	if e.structureVersion == e.buffer.Version && len(e.insideFence) == len(e.buffer.Lines) {
		return e.insideFence, e.delimiter, e.metadata
	}
	end := len(e.buffer.Lines)
	inside = make([]bool, end)
	delimiter = make([]bool, end)
	metadata = make([]bool, end)
	inFence := false
	var mark rune
	markerLen := 0
	metaMark := ""
	for n := 0; n < end; n++ {
		line := string(e.buffer.Lines[n])
		if n == 0 && (line == "---" || line == "+++") {
			metaMark = line
			metadata[n] = true
			continue
		}
		if metaMark != "" {
			metadata[n] = true
			if line == metaMark {
				metaMark = ""
			}
			continue
		}
		m, count, fence := fenceMarker(e.buffer.Lines[n])
		inside[n], delimiter[n] = inFence, fence
		if fence {
			if !inFence {
				inFence, mark, markerLen = true, m, count
			} else if m == mark && count >= markerLen {
				inFence = false
			}
		}
	}
	e.structureVersion = e.buffer.Version
	e.insideFence, e.delimiter, e.metadata = inside, delimiter, metadata
	return
}
func thematic(rs []rune) bool {
	mark := rune(0)
	count := 0
	for _, r := range rs {
		if r == ' ' || r == '\t' {
			continue
		}
		if mark == 0 && (r == '-' || r == '*' || r == '_') {
			mark = r
		}
		if r != mark {
			return false
		}
		count++
	}
	return count >= 3
}
func setextDelimiter(rs []rune) bool {
	count := 0
	mark := rune(0)
	for _, r := range rs {
		if r == ' ' || r == '\t' {
			continue
		}
		if mark == 0 && (r == '=' || r == '-') {
			mark = r
		}
		if r != mark {
			return false
		}
		count++
	}
	return count > 0
}
func (e *Editor) structuralStyles(line int, rs []rune, insideFence, delimiter, metadata bool) []string {
	styles := make([]string, len(rs))
	if metadata {
		fillStyle(styles, 0, len(styles), "\x1b[38;5;109m")
		return styles
	}
	if insideFence {
		fillStyle(styles, 0, len(styles), "\x1b[38;5;109m")
		return styles
	}
	if delimiter {
		fillStyle(styles, 0, len(styles), "\x1b[38;5;244m")
		return styles
	}
	if len(rs) >= 4 && string(rs[:4]) == "    " {
		fillStyle(styles, 0, len(styles), "\x1b[38;5;109m")
		return styles
	}
	if thematic(rs) || setextDelimiter(rs) {
		fillStyle(styles, 0, len(styles), "\x1b[1;38;5;141m")
	}
	if line+1 < len(e.buffer.Lines) && setextDelimiter(e.buffer.Lines[line+1]) && strings.TrimSpace(string(rs)) != "" {
		fillStyle(styles, 0, len(styles), "\x1b[1;38;5;141m")
	}
	trim := 0
	for trim < len(rs) && (rs[trim] == ' ' || rs[trim] == '\t') {
		trim++
	}
	if trim < len(rs) {
		if rs[trim] == '<' {
			fillStyle(styles, trim, len(styles), "\x1b[38;5;109m")
		}
		if rs[trim] == '[' {
			for j := trim + 1; j+1 < len(rs); j++ {
				if rs[j] == ']' && rs[j+1] == ':' {
					fillStyle(styles, trim, j+1, "\x1b[38;5;141m")
					fillStyle(styles, j+2, len(styles), "\x1b[38;5;75m")
					break
				}
			}
		}
		switch rs[trim] {
		case '>':
			fillStyle(styles, trim, trim+1, "\x1b[38;5;244m")
		case '-', '*', '+':
			if trim+1 < len(rs) && (rs[trim+1] == ' ' || rs[trim+1] == '\t') {
				fillStyle(styles, trim, trim+1, "\x1b[38;5;220m")
			}
		}
		if rs[trim] >= '0' && rs[trim] <= '9' {
			j := trim
			for j < len(rs) && rs[j] >= '0' && rs[j] <= '9' {
				j++
			}
			if j < len(rs) && (rs[j] == '.' || rs[j] == ')') {
				fillStyle(styles, trim, j+1, "\x1b[38;5;220m")
			}
		}
	}
	for i, r := range rs {
		if r == '|' {
			styles[i] = "\x1b[38;5;244m"
		}
	}
	return styles
}
func (e *Editor) stylesFor(line int, rs []rune, insideFence, delimiter, metadata bool) []string {
	if e.plainText {
		return make([]string, len(rs))
	}
	styles := e.structuralStyles(line, rs, insideFence, delimiter, metadata)
	for _, t := range e.tokens {
		if t.Line != line {
			continue
		}
		a := runeIndexUTF16(rs, t.Start)
		b := runeIndexUTF16(rs, t.Start+t.Length)
		style := tokenStyle(t.Kind)
		for i := a; i < b && i < len(styles); i++ {
			styles[i] = style
		}
	}
	return styles
}

func safeDisplay(cluster string) (string, int) {
	for _, r := range cluster {
		if r < 32 || r >= 0x7f && r <= 0x9f {
			return "�", 1
		}
	}
	w := runewidth.StringWidth(cluster)
	if w < 1 {
		w = 1
	}
	return cluster, w
}

func renderContent(rs []rune, styles []string, start, maxWidth int) string {
	var b strings.Builder
	cell := 0
	used := 0
	active := ""
	runeAt := 0
	g := uniseg.NewGraphemes(string(rs))
	for g.Next() {
		cluster := g.Str()
		count := len([]rune(cluster))
		display, w := safeDisplay(cluster)
		if cluster == "\t" {
			w = 4 - cell%4
			display = strings.Repeat(" ", w)
		}
		if cell+w <= start {
			cell += w
			runeAt += count
			continue
		}
		if cell < start {
			blank := cell + w - start
			if blank <= maxWidth {
				b.WriteString(strings.Repeat(" ", blank))
				used += blank
			}
			cell += w
			runeAt += count
			continue
		}
		if used+w > maxWidth {
			break
		}
		style := ""
		if runeAt < len(styles) {
			style = styles[runeAt]
		}
		if style != active {
			if style == "" {
				b.WriteString("\x1b[0m")
			} else {
				b.WriteString(style)
			}
			active = style
		}
		b.WriteString(display)
		used += w
		cell += w
		runeAt += count
	}
	b.WriteString("\x1b[0m")
	return b.String()
}
func formatElapsed(elapsed time.Duration) string {
	if elapsed < 0 {
		elapsed = 0
	}
	seconds := int(elapsed / time.Second)
	hours := seconds / 3600
	minutes := seconds % 3600 / 60
	seconds %= 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func (e *Editor) statusBar(now time.Time) string {
	status := e.status
	if e.buffer.Dirty {
		status = "● " + status
	}
	name := filepathBase(e.buffer.Path)
	if e.anonymous {
		name = "[No Name]"
	}
	mode := "OFF"
	if e.keepOpen {
		mode = "ON"
	}
	status = " Ctrl+K Keep open: " + mode + " | " + name + "  " + status

	width := max(0, e.cols-1)
	elapsed := time.Duration(0)
	if !e.openedAt.IsZero() {
		elapsed = now.Sub(e.openedAt)
	}
	timer := clipPlain("Time "+formatElapsed(elapsed), width)
	remaining := width - visualWidth([]rune(timer))
	left := clipPlain(status, remaining)
	return left + strings.Repeat(" ", max(0, remaining-visualWidth([]rune(left)))) + timer
}

func clipPlain(s string, width int) string {
	if width <= 0 {
		return ""
	}
	var b strings.Builder
	used := 0
	g := uniseg.NewGraphemes(s)
	for g.Next() {
		display, w := safeDisplay(g.Str())
		if used+w > width {
			break
		}
		b.WriteString(display)
		used += w
	}
	return b.String()
}

func (e *Editor) frame() string {
	gutter, editorRows, caretRow, caretCol := e.layout()
	var frame strings.Builder
	frame.Grow(e.rows*e.cols + 256)
	frame.WriteString("\x1b[?25l")
	insideFence, delimiter, metadata := e.structuralStates()
	for screen := 0; screen < editorRows; screen++ {
		fmt.Fprintf(&frame, "\x1b[%d;1H\x1b[2K", screen+1)
		line := e.top + screen
		if line >= len(e.buffer.Lines) {
			continue
		}
		if gutter > 0 {
			fmt.Fprintf(&frame, "\x1b[38;5;240m%*d \x1b[0m", gutter-1, line+1)
		}
		rs := e.buffer.Lines[line]
		lineWidth := e.cols - gutter
		if screen == e.rows-1 {
			lineWidth--
		}
		frame.WriteString(renderContent(rs, e.stylesFor(line, rs, insideFence[line], delimiter[line], metadata[line]), e.hscroll, max(0, lineWidth)))
	}
	if e.rows >= 2 {
		fmt.Fprintf(&frame, "\x1b[%d;1H\x1b[2K\x1b[7m%s\x1b[0m", e.rows, e.statusBar(time.Now()))
	}
	e.renderPopup(&frame, caretRow, caretCol, editorRows)
	focusRow, focusCol := caretRow, caretCol
	if e.gotoOpen {
		prompt := " Ir para linha:coluna: " + e.gotoInput
		if e.gotoError != "" {
			prompt = " " + e.gotoError + " " + e.gotoInput
		}
		prompt = clipPlain(prompt, max(0, e.cols-1))
		fmt.Fprintf(&frame, "\x1b[%d;1H\x1b[2K\x1b[7m%s\x1b[0m", e.rows, prompt)
		focusRow = e.rows - 1
		focusCol = min(visualWidth([]rune(prompt)), e.cols-1)
	} else if e.confirmCancel {
		prompt := clipPlain(" Descartar e sair? [y/N]", max(0, e.cols-1))
		fmt.Fprintf(&frame, "\x1b[%d;1H\x1b[2K\x1b[7m%s\x1b[0m", e.rows, prompt)
		focusRow = e.rows - 1
		focusCol = min(visualWidth([]rune(prompt)), e.cols-1)
	}
	fmt.Fprintf(&frame, "\x1b[%d;%dH\x1b[?25h", focusRow+1, focusCol+1)
	return frame.String()
}
func (e *Editor) render() error { _, err := io.WriteString(e.output, e.frame()); return err }

func (e *Editor) renderElapsed() error {
	if e.rows < 2 || e.confirmCancel || e.gotoOpen {
		return nil
	}
	_, _, caretRow, caretCol := e.layout()
	var frame strings.Builder
	frame.Grow(e.cols + 64)
	frame.WriteString("\x1b[?25l")
	fmt.Fprintf(&frame, "\x1b[%d;1H\x1b[2K\x1b[7m%s\x1b[0m", e.rows, e.statusBar(time.Now()))
	fmt.Fprintf(&frame, "\x1b[%d;%dH\x1b[?25h", caretRow+1, caretCol+1)
	_, err := io.WriteString(e.output, frame.String())
	return err
}
func filepathBase(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}
func completionWindow(selected, total, count int) int {
	return max(0, min(selected-count/2, total-count))
}
func (e *Editor) renderPopup(frame *strings.Builder, caretRow, caretCol, editorRows int) {
	if len(e.completions) == 0 || e.rows < 2 || e.confirmCancel {
		return
	}
	count := min(8, len(e.completions))
	row := caretRow + 1
	if row+count > editorRows {
		row = max(0, caretRow-count)
	}
	width := min(48, e.cols-caretCol)
	if width < 4 {
		return
	}
	col := caretCol
	start := completionWindow(e.selected, len(e.completions), count)
	for visible := 0; visible < count; visible++ {
		i := start + visible
		fmt.Fprintf(frame, "\x1b[%d;%dH", row+visible+1, col+1)
		label := clipPlain(e.completions[i].Label, width-2)
		label += strings.Repeat(" ", max(0, width-2-runewidth.StringWidth(label)))
		if i == e.selected {
			fmt.Fprintf(frame, "\x1b[48;5;238;38;5;255m %s \x1b[0m", label)
		} else {
			fmt.Fprintf(frame, "\x1b[48;5;235;38;5;252m %s \x1b[0m", label)
		}
	}
}

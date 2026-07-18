package editor

import (
	"bytes"
	"io"
	"testing"
)

func TestDecoderFragmentedSequencesAndPaste(t *testing.T) {
	d := new(inputDecoder)
	var keys []Key
	for _, part := range [][]byte{[]byte("\x1b"), []byte("[A\x1b["), []byte("3~\x1b[200~hello\n"), []byte("world\x1b[20"), []byte("1~")} {
		keys = append(keys, d.Feed(part)...)
	}
	if len(keys) != 3 || keys[0].Kind != KeyUp || keys[1].Kind != KeyDelete || keys[2].Kind != KeyPaste || keys[2].Text != "hello\nworld" {
		t.Fatalf("keys=%#v", keys)
	}
}
func TestDecoderPreservesCharacterAfterEscape(t *testing.T) {
	d := new(inputDecoder)
	keys := d.Feed([]byte("\x1bx"))
	if len(keys) != 2 || keys[0].Kind != KeyEscape || keys[1].Rune != 'x' {
		t.Fatalf("keys=%#v", keys)
	}
	d = new(inputDecoder)
	if got := d.Feed([]byte("\x1b")); len(got) != 0 {
		t.Fatalf("premature escape=%#v", got)
	}
	keys = d.FlushEscape()
	if len(keys) != 1 || keys[0].Kind != KeyEscape {
		t.Fatalf("flush=%#v", keys)
	}
}
func TestWordDeleteAndCtrlEnterSequences(t *testing.T) {
	d := new(inputDecoder)
	keys := d.Feed([]byte{23, 0x1b, 127})
	if len(keys) != 2 || keys[0].Kind != KeyDeleteWord || keys[1].Kind != KeyDeleteWord {
		t.Fatalf("word keys=%#v", keys)
	}
	d = new(inputDecoder)
	keys = d.Feed([]byte{8})
	if len(keys) != 1 || keys[0].Kind != KeyBackspace {
		t.Fatalf("ambiguous BS key=%#v", keys)
	}
	d = new(inputDecoder)
	keys = d.Feed([]byte("\x1b[27;5;127~"))
	if len(keys) != 1 || keys[0].Kind != KeyDeleteWord {
		t.Fatalf("xterm word key=%#v", keys)
	}
	d = new(inputDecoder)
	keys = d.Feed([]byte("\x1b[13;5u\x1b[27;5;13~"))
	if len(keys) != 2 || keys[0].Kind != KeyInsertLineBelow || keys[1].Kind != KeyInsertLineBelow {
		t.Fatalf("ctrl-enter keys=%#v", keys)
	}
	d = new(inputDecoder)
	keys = d.Feed([]byte{'\n'})
	if len(keys) != 1 || keys[0].Kind != KeyInsertLineBelow {
		t.Fatalf("LF key=%#v", keys)
	}
}

func TestStartInputEmitsBeforeEOF(t *testing.T) {
	keys, errs := startInput(bytes.NewBufferString("a"))
	if key := <-keys; key.Kind != KeyRune || key.Rune != 'a' {
		t.Fatalf("key=%#v", key)
	}
	if err := <-errs; err != io.EOF {
		t.Fatalf("err=%v", err)
	}
}
func TestDisplayWidth(t *testing.T) {
	if got := visualWidth([]rune("a\t界e\u0301")); got != 7 {
		t.Fatalf("width=%d want 7", got)
	}
}

func TestRendererEscapesUntrustedControls(t *testing.T) {
	text := []rune("safe\x1b]52;c;secret\a\u009b31m")
	got := renderContent(text, make([]string, len(text)), 0, 200)
	if bytes.Contains([]byte(got), []byte("\x1b]52")) || bytes.Contains([]byte(got), []byte("\u009b")) || bytes.Contains([]byte(got), []byte("\a")) {
		t.Fatalf("control leaked: %q", got)
	}
	plain := clipPlain("file\x1b]0;owned\a.prompt", 80)
	if bytes.Contains([]byte(plain), []byte("\x1b")) || bytes.Contains([]byte(plain), []byte("\a")) {
		t.Fatalf("plain control leaked: %q", plain)
	}
}

func TestSmallLayoutAndStructuralStates(t *testing.T) {
	e := &Editor{buffer: &Buffer{Lines: [][]rune{[]rune("---"), []rune("key: value"), []rune("---"), []rune("```go"), []rune("- not a list"), []rune("```")}}, rows: 1, cols: 1}
	gutter, rows, _, col := e.layout()
	if gutter != 0 || rows != 1 || col != 0 {
		t.Fatalf("layout gutter=%d rows=%d col=%d", gutter, rows, col)
	}
	inside, delim, meta := e.structuralStates()
	if !meta[1] || !delim[3] || !inside[4] {
		t.Fatalf("states inside=%v delim=%v metadata=%v", inside, delim, meta)
	}
	styles := e.structuralStyles(4, e.buffer.Lines[4], inside[4], delim[4], meta[4])
	if len(styles) == 0 || styles[0] != "\x1b[38;5;109m" {
		t.Fatalf("fenced list style=%q", styles)
	}
}

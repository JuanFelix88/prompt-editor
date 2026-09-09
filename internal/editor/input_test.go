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
func TestModifiedKeySequences(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want KeyKind
	}{
		{"ctrl-w", []byte{23}, KeyDeleteWord},
		{"kitty-ctrl-w", []byte("\x1b[119;5u"), KeyDeleteWord},
		{"kitty-ctrl-s", []byte("\x1b[115;5u"), KeySave},
		{"ctrl-g", []byte{7}, KeyGoTo},
		{"kitty-ctrl-g", []byte("\x1b[103;5u"), KeyGoTo},
		{"xterm-ctrl-g", []byte("\x1b[27;5;103~"), KeyGoTo},
		{"ctrl-k", []byte{11}, KeyToggleKeepOpen},
		{"kitty-ctrl-k", []byte("\x1b[107;5u"), KeyToggleKeepOpen},
		{"xterm-ctrl-k", []byte("\x1b[27;5;107~"), KeyToggleKeepOpen},
		{"xterm-ctrl-s", []byte("\x1b[27;5;115~"), KeySave},
		{"xterm-ctrl-x", []byte("\x1b[27;5;120~"), KeyCancel},
		{"xterm-ctrl-space", []byte("\x1b[27;5;32~"), KeyComplete},
		{"kitty-enter", []byte("\x1b[13u"), KeyEnter},
		{"kitty-escape", []byte("\x1b[27u"), KeyEscape},
		{"kitty-backspace", []byte("\x1b[127u"), KeyBackspace},
		{"alt-backspace", []byte{0x1b, 127}, KeyDeleteWord},
		{"ctrl-backspace", []byte("\x1b[27;5;127~"), KeyDeleteWord},
		{"ctrl-delete", []byte("\x1b[3;5~"), KeyDeleteWordForward},
		{"alt-left-csi", []byte("\x1b[1;3D"), KeyWordLeft},
		{"ctrl-left", []byte("\x1b[1;5D"), KeyWordLeft},
		{"alt-left-meta", []byte("\x1bb"), KeyWordLeft},
		{"alt-left-kitty", []byte("\x1b[98;3u"), KeyWordLeft},
		{"alt-left-xterm", []byte("\x1b[27;3;98~"), KeyWordLeft},
		{"alt-right-csi", []byte("\x1b[1;3C"), KeyWordRight},
		{"ctrl-right", []byte("\x1b[1;5C"), KeyWordRight},
		{"alt-right-meta", []byte("\x1bf"), KeyWordRight},
		{"ctrl-home-csi", []byte("\x1b[1;5H"), KeyDocumentHome},
		{"ctrl-home-rxvt", []byte("\x1b[7;5~"), KeyDocumentHome},
		{"ctrl-end-csi", []byte("\x1b[1;5F"), KeyDocumentEnd},
		{"ctrl-end-rxvt", []byte("\x1b[8;5~"), KeyDocumentEnd},
		{"alt-right-kitty", []byte("\x1b[102;3u"), KeyWordRight},
		{"ctrl-shift-s-csi-u", []byte("\x1b[115;6u"), KeySaveWithoutExit},
		{"ctrl-shift-s-xterm", []byte("\x1b[27;6;83~"), KeySaveWithoutExit},
		{"alt-up", []byte("\x1b[1;3A"), KeyMoveLineUp},
		{"alt-down", []byte("\x1b[1;3B"), KeyMoveLineDown},
		{"ctrl-enter", []byte("\x1b[13;5u"), KeyInsertLineBelow},
		{"shift-enter", []byte("\x1b[13;2u"), KeyInsertLineAbove},
		{"legacy-ctrl-enter", []byte{'\n'}, KeyInsertLineBelow},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			keys := new(inputDecoder).Feed(test.data)
			if len(keys) != 1 || keys[0].Kind != test.want {
				t.Fatalf("keys=%#v want kind=%v", keys, test.want)
			}
		})
	}

	keys := new(inputDecoder).Feed([]byte{8})
	if len(keys) != 1 || keys[0].Kind != KeyBackspace {
		t.Fatalf("ambiguous BS key=%#v", keys)
	}
}

func TestModifiedPrintableRuneSequences(t *testing.T) {
	for name, test := range map[string]struct {
		data []byte
		want rune
	}{
		"xterm-shift-letter": {[]byte("\x1b[27;2;65~"), 'A'},
		"kitty-alt-letter":   {[]byte("\x1b[120;3u"), 'x'},
	} {
		t.Run(name, func(t *testing.T) {
			keys := new(inputDecoder).Feed(test.data)
			if len(keys) != 1 || keys[0].Kind != KeyRune || keys[0].Rune != test.want {
				t.Fatalf("keys=%#v want rune=%q", keys, test.want)
			}
		})
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

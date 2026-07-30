package editor

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type KeyKind int

const (
	KeyRune KeyKind = iota
	KeyLeft
	KeyRight
	KeyUp
	KeyDown
	KeyHome
	KeyEnd
	KeyDelete
	KeyBackspace
	KeyEnter
	KeyTab
	KeyEscape
	KeySave
	KeyCancel
	KeyComplete
	KeyPageUp
	KeyPageDown
	KeyPaste
	KeySuspend
	KeyDeleteWord
	KeyInsertLineBelow
	KeyDeleteWordForward
	KeyWordLeft
	KeyWordRight
	KeySaveWithoutExit
	KeyMoveLineUp
	KeyMoveLineDown
	KeyInsertLineAbove
)

type Key struct {
	Kind KeyKind
	Rune rune
	Text string
}

var pasteEnd = []byte("\x1b[201~")

type inputDecoder struct {
	buf     []byte
	paste   bytes.Buffer
	inPaste bool
}

func (d *inputDecoder) Feed(data []byte) []Key {
	d.buf = append(d.buf, data...)
	return d.decode()
}

func (d *inputDecoder) decode() (keys []Key) {
	for len(d.buf) > 0 {
		if d.inPaste {
			if at := bytes.Index(d.buf, pasteEnd); at >= 0 {
				d.paste.Write(d.buf[:at])
				d.buf = d.buf[at+len(pasteEnd):]
				keys = append(keys, Key{Kind: KeyPaste, Text: d.paste.String()})
				d.paste.Reset()
				d.inPaste = false
				continue
			}
			keep := len(pasteEnd) - 1
			if len(d.buf) > keep {
				d.paste.Write(d.buf[:len(d.buf)-keep])
				d.buf = d.buf[len(d.buf)-keep:]
			}
			return keys
		}

		b := d.buf[0]
		if b == 0x1b {
			if len(d.buf) == 1 {
				return keys
			}
			switch d.buf[1] {
			case 8, 127:
				keys = append(keys, Key{Kind: KeyDeleteWord})
				d.buf = d.buf[2:]
				continue
			case 'b':
				keys = append(keys, Key{Kind: KeyWordLeft})
				d.buf = d.buf[2:]
				continue
			case 'f':
				keys = append(keys, Key{Kind: KeyWordRight})
				d.buf = d.buf[2:]
				continue
			}
			if d.buf[1] != '[' && d.buf[1] != 'O' {
				keys = append(keys, Key{Kind: KeyEscape})
				d.buf = d.buf[1:]
				continue
			}
			end := -1
			for i := 2; i < len(d.buf) && i < 16; i++ {
				if d.buf[i] >= 'A' && d.buf[i] <= 'Z' || d.buf[i] >= 'a' && d.buf[i] <= 'z' || d.buf[i] == '~' {
					end = i
					break
				}
			}
			if end < 0 {
				if len(d.buf) >= 16 {
					keys = append(keys, Key{Kind: KeyEscape})
					d.buf = d.buf[1:]
					continue
				}
				return keys
			}
			seq := string(d.buf[2 : end+1])
			d.buf = d.buf[end+1:]
			if seq == "200~" {
				d.inPaste = true
				continue
			}
			if key, ok := escapeKey(seq); ok {
				keys = append(keys, key)
			} else if r, ok := modifiedRune(seq); ok {
				keys = append(keys, Key{Kind: KeyRune, Rune: r})
			} else {
				keys = append(keys, Key{Kind: KeyEscape})
			}
			continue
		}
		switch b {
		case 0:
			keys = append(keys, Key{Kind: KeyComplete})
			d.buf = d.buf[1:]
			continue
		case 3, 24:
			keys = append(keys, Key{Kind: KeyCancel})
			d.buf = d.buf[1:]
			continue
		case 19:
			keys = append(keys, Key{Kind: KeySave})
			d.buf = d.buf[1:]
			continue
		case 23:
			keys = append(keys, Key{Kind: KeyDeleteWord})
			d.buf = d.buf[1:]
			continue
		case 26:
			keys = append(keys, Key{Kind: KeySuspend})
			d.buf = d.buf[1:]
			continue
		case '\r':
			keys = append(keys, Key{Kind: KeyEnter})
			d.buf = d.buf[1:]
			continue
		case '\n':
			keys = append(keys, Key{Kind: KeyInsertLineBelow})
			d.buf = d.buf[1:]
			continue
		case '\t':
			keys = append(keys, Key{Kind: KeyTab})
			d.buf = d.buf[1:]
			continue
		case 8:
			keys = append(keys, Key{Kind: KeyBackspace})
			d.buf = d.buf[1:]
			continue
		case 127:
			keys = append(keys, Key{Kind: KeyBackspace})
			d.buf = d.buf[1:]
			continue
		}
		if b < 32 {
			d.buf = d.buf[1:]
			continue
		}
		if !utf8.FullRune(d.buf) {
			return keys
		}
		r, size := utf8.DecodeRune(d.buf)
		keys = append(keys, Key{Kind: KeyRune, Rune: r})
		d.buf = d.buf[size:]
	}
	return keys
}

func escapeKey(seq string) (Key, bool) {
	switch seq {
	case "A":
		return Key{Kind: KeyUp}, true
	case "B":
		return Key{Kind: KeyDown}, true
	case "C":
		return Key{Kind: KeyRight}, true
	case "D":
		return Key{Kind: KeyLeft}, true
	case "H", "1~", "7~":
		return Key{Kind: KeyHome}, true
	case "F", "4~", "8~":
		return Key{Kind: KeyEnd}, true
	case "3~":
		return Key{Kind: KeyDelete}, true
	case "5~":
		return Key{Kind: KeyPageUp}, true
	case "6~":
		return Key{Kind: KeyPageDown}, true
	case "27u":
		return Key{Kind: KeyEscape}, true
	case "13u":
		return Key{Kind: KeyEnter}, true
	case "9u":
		return Key{Kind: KeyTab}, true
	case "8u", "127u":
		return Key{Kind: KeyBackspace}, true
	case "32;5u", "27;5;32~":
		return Key{Kind: KeyComplete}, true
	case "99;5u", "120;5u", "27;5;99~", "27;5;120~":
		return Key{Kind: KeyCancel}, true
	case "115;5u", "27;5;115~":
		return Key{Kind: KeySave}, true
	case "122;5u", "27;5;122~":
		return Key{Kind: KeySuspend}, true
	case "8;5u", "119;5u", "127;5u", "27;3;8~", "27;3;127~", "27;5;8~", "27;5;119~", "27;5;127~":
		return Key{Kind: KeyDeleteWord}, true
	case "3;5~", "27;5;46~":
		return Key{Kind: KeyDeleteWordForward}, true
	case "1;3D", "1;5D", "98;3u", "27;3;98~":
		return Key{Kind: KeyWordLeft}, true
	case "1;3C", "1;5C", "102;3u", "27;3;102~":
		return Key{Kind: KeyWordRight}, true
	case "115;6u", "83;6u", "27;6;115~", "27;6;83~":
		return Key{Kind: KeySaveWithoutExit}, true
	case "1;3A":
		return Key{Kind: KeyMoveLineUp}, true
	case "1;3B":
		return Key{Kind: KeyMoveLineDown}, true
	case "13;5u", "27;5;13~":
		return Key{Kind: KeyInsertLineBelow}, true
	case "13;2u", "27;2;13~":
		return Key{Kind: KeyInsertLineAbove}, true
	default:
		return Key{}, false
	}
}

func modifiedRune(seq string) (rune, bool) {
	var codeText, modifierText string
	if strings.HasSuffix(seq, "~") {
		parts := strings.Split(strings.TrimSuffix(seq, "~"), ";")
		if len(parts) != 3 || parts[0] != "27" {
			return 0, false
		}
		modifierText, codeText = parts[1], parts[2]
	} else if strings.HasSuffix(seq, "u") {
		parts := strings.Split(strings.TrimSuffix(seq, "u"), ";")
		if len(parts) != 2 {
			return 0, false
		}
		codeText, modifierText = parts[0], parts[1]
	} else {
		return 0, false
	}
	modifier, err := strconv.Atoi(modifierText)
	if err != nil || modifier < 2 || (modifier-1)&4 != 0 {
		return 0, false
	}
	code, err := strconv.ParseInt(codeText, 10, 32)
	r := rune(code)
	if err != nil || !utf8.ValidRune(r) || r < 32 || r == 127 {
		return 0, false
	}
	return r, true
}

func (d *inputDecoder) WaitingEscape() bool { return !d.inPaste && len(d.buf) > 0 && d.buf[0] == 0x1b }
func (d *inputDecoder) FlushEscape() []Key {
	if !d.WaitingEscape() {
		return nil
	}
	d.buf = d.buf[1:]
	return append([]Key{{Kind: KeyEscape}}, d.decode()...)
}

type inputChunk struct {
	data []byte
	err  error
}

func startInput(r io.Reader) (<-chan Key, <-chan error) {
	keys := make(chan Key, 16)
	errs := make(chan error, 1)
	chunks := make(chan inputChunk, 4)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				copyOf := append([]byte(nil), buf[:n]...)
				chunks <- inputChunk{data: copyOf}
			}
			if err != nil {
				chunks <- inputChunk{err: err}
				return
			}
		}
	}()
	go func() {
		decoder := new(inputDecoder)
		var timer *time.Timer
		var timeout <-chan time.Time
		reset := func() {
			if timer != nil && !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer = time.NewTimer(30 * time.Millisecond)
			timeout = timer.C
		}
		emit := func(items []Key) {
			for _, key := range items {
				keys <- key
			}
		}
		for {
			select {
			case chunk := <-chunks:
				if len(chunk.data) > 0 {
					emit(decoder.Feed(chunk.data))
					if decoder.WaitingEscape() {
						reset()
					} else {
						timeout = nil
					}
				}
				if chunk.err != nil {
					emit(decoder.FlushEscape())
					errs <- chunk.err
					return
				}
			case <-timeout:
				emit(decoder.FlushEscape())
				timeout = nil
			}
		}
	}()
	return keys, errs
}

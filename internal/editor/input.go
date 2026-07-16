package editor

import (
	"bytes"
	"io"
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
		case 26:
			keys = append(keys, Key{Kind: KeySuspend})
			d.buf = d.buf[1:]
			continue
		case '\r', '\n':
			keys = append(keys, Key{Kind: KeyEnter})
			d.buf = d.buf[1:]
			continue
		case '\t':
			keys = append(keys, Key{Kind: KeyTab})
			d.buf = d.buf[1:]
			continue
		case 8, 127:
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
	default:
		return Key{}, false
	}
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

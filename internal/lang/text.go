package lang

import (
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

func splitLines(text string) []string {
	return strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
}

func UTF16Len(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xffff {
			n += 2
		} else {
			n++
		}
	}
	return n
}

func utf16ToByte(s string, units int) int {
	if units <= 0 {
		return 0
	}
	u := 0
	for i, r := range s {
		w := 1
		if r > 0xffff {
			w = 2
		}
		if u+w > units {
			return i
		}
		u += w
		if u == units {
			return i + utf8.RuneLen(r)
		}
	}
	return len(s)
}

func runeToUTF16(rs []rune, index int) int {
	if index > len(rs) {
		index = len(rs)
	}
	if index < 0 {
		index = 0
	}
	return len(utf16.Encode(rs[:index]))
}

func isPromptPath(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return name == "prompt.md" || name == "prompts.md" ||
		strings.HasSuffix(name, ".prompt.md") || strings.HasSuffix(name, ".prompts.md") ||
		strings.HasSuffix(name, ".prompt")
}

func pathEqualOrInside(root, target string) bool {
	if runtime.GOOS == "windows" {
		root, target = strings.ToLower(root), strings.ToLower(target)
	}
	return target == root || strings.HasPrefix(target, root+string(filepath.Separator))
}

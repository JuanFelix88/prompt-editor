package editor

import (
	"testing"

	"github.com/juanandrade/prompt-cli/internal/lang"
)

func TestPromptItemStylesUseTerminalThemeColors(t *testing.T) {
	if got := tokenStyle(lang.TokenPromptItem); got != "\x1b[31m" {
		t.Fatalf("hyphen item style=%q, want ANSI theme red", got)
	}
	if got := tokenStyle(lang.TokenPromptStarItem); got != "\x1b[34m" {
		t.Fatalf("star item style=%q, want ANSI theme blue", got)
	}
}

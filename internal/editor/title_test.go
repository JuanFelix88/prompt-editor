package editor

import (
	"path/filepath"
	"testing"
)

func TestTerminalTitle(t *testing.T) {
	tests := []struct {
		name string
		e    *Editor
		want string
	}{
		{
			name: "file",
			e:    &Editor{buffer: &Buffer{Path: filepath.Join("workspace", "review.prompt")}},
			want: "\x1b]0;prompt-cli — review.prompt\x07",
		},
		{
			name: "untitled",
			e:    &Editor{buffer: &Buffer{}, anonymous: true},
			want: "\x1b]0;prompt-cli — Untitled\x07",
		},
		{
			name: "control characters are removed",
			e:    &Editor{buffer: &Buffer{Path: "unsafe\x1b]0;title.prompt"}},
			want: "\x1b]0;prompt-cli — unsafe]0;title.prompt\x07",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.e.terminalTitle(); got != tt.want {
				t.Fatalf("terminalTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

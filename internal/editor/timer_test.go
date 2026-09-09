package editor

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestFormatElapsed(t *testing.T) {
	tests := []struct {
		elapsed time.Duration
		want    string
	}{
		{-time.Second, "00:00"},
		{0, "00:00"},
		{62 * time.Second, "01:02"},
		{time.Hour + 2*time.Minute + 3*time.Second, "1:02:03"},
	}

	for _, test := range tests {
		if got := formatElapsed(test.elapsed); got != test.want {
			t.Errorf("formatElapsed(%s) = %q, want %q", test.elapsed, got, test.want)
		}
	}
}

func TestElapsedRenderOnlyUpdatesStatusBar(t *testing.T) {
	output, err := os.CreateTemp(t.TempDir(), "editor-output")
	if err != nil {
		t.Fatal(err)
	}
	e := &Editor{
		buffer: &Buffer{Path: "prompt.md", Lines: [][]rune{[]rune("confidential buffer text")}},
		cols:   80,
		rows:   24,
		status: "ready",
		output: output,
	}
	if err := e.renderElapsed(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output.Name())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "confidential buffer text") {
		t.Fatalf("elapsed render included buffer content: %q", data)
	}
	if !strings.Contains(string(data), "Time 00:00") {
		t.Fatalf("elapsed render omitted timer: %q", data)
	}
}

func TestStatusBarPlacesElapsedTimeAtRight(t *testing.T) {
	now := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)
	e := &Editor{
		buffer:   &Buffer{Path: "prompt.md", Lines: [][]rune{{}}},
		cols:     80,
		status:   "ready",
		openedAt: now.Add(-62 * time.Second),
	}

	want := " Ctrl+K Keep open: OFF | prompt.md  ready" + strings.Repeat(" ", 28) + "Time 01:02"
	if got := e.statusBar(now); got != want {
		t.Fatalf("statusBar() = %q, want %q", got, want)
	}
}

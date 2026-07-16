package lsp

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func frame(body string) string { return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body) }

func TestServerLifecycleAndUnknownMethod(t *testing.T) {
	input := frame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		frame(`{"jsonrpc":"2.0","id":2,"method":"unknown","params":{}}`) +
		frame(`{"jsonrpc":"2.0","id":3,"method":"shutdown","params":null}`) +
		frame(`{"jsonrpc":"2.0","method":"exit"}`)
	var out, logs bytes.Buffer
	s, err := New(t.TempDir(), true, strings.NewReader(input), &out, &logs)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Run(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, `"completionProvider"`) {
		t.Fatalf("initialize response missing capabilities: %s", got)
	}
	if !strings.Contains(got, `"code":-32601`) {
		t.Fatalf("unknown method response missing: %s", got)
	}
	if !strings.Contains(got, `"id":3,"jsonrpc":"2.0","result":null`) && !strings.Contains(got, `"result":null`) {
		t.Fatalf("shutdown must return null: %s", got)
	}
	if logs.Len() != 0 {
		t.Fatalf("unexpected logs: %s", logs.String())
	}
}
func TestOversizedFrameIsRejectedBeforeAllocation(t *testing.T) {
	input := fmt.Sprintf("Content-Length: %d\r\n\r\n", maxMessageBytes+1)
	var out, logs bytes.Buffer
	s, _ := New(t.TempDir(), true, strings.NewReader(input), &out, &logs)
	if err := s.Run(); !strings.Contains(fmt.Sprint(err), "16 MiB") {
		t.Fatalf("err=%v", err)
	}
}

func TestMalformedJSONStaysOutOfProtocolOutput(t *testing.T) {
	input := frame(`not json`) + frame(`{"jsonrpc":"2.0","method":"exit"}`)
	var out, logs bytes.Buffer
	s, _ := New(t.TempDir(), true, strings.NewReader(input), &out, &logs)
	_ = s.Run()
	if out.Len() != 0 {
		t.Fatalf("malformed JSON leaked to stdout: %q", out.String())
	}
	if !strings.Contains(logs.String(), "lsp json") {
		t.Fatalf("missing stderr log: %q", logs.String())
	}
}

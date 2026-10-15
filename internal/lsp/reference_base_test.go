package lsp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/juanandrade/prompt-cli/internal/lang"
)

func TestLSPReferencesIgnoreWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "local.md")
	if err := os.WriteFile(target, []byte("fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, explicit := range []bool{false, true} {
		name := "client-workspace"
		if explicit {
			name = "explicit-workspace"
		}
		t.Run(name, func(t *testing.T) {
			var out, logs bytes.Buffer
			s, err := New(t.TempDir(), explicit, strings.NewReader(""), &out, &logs)
			if err != nil {
				t.Fatal(err)
			}
			request := func(method, params string) any {
				t.Helper()
				result, known, err := s.handle(message{Method: method, Params: json.RawMessage(params)})
				if err != nil || !known {
					t.Fatalf("%s: known=%v err=%v", method, known, err)
				}
				return result
			}
			initialize, _ := json.Marshal(map[string]string{"rootUri": lang.PathToURI(root)})
			request("initialize", string(initialize))
			uri := lang.PathToURI(filepath.Join(dir, "prompt.md"))
			params, _ := json.Marshal(map[string]any{
				"textDocument": map[string]string{"uri": uri, "text": "@local.md"},
				"position":     lang.Position{Character: 2},
			})
			request("textDocument/didOpen", string(params))
			completion := request("textDocument/completion", string(params)).(lang.CompletionList)
			if len(completion.Items) != 1 || completion.Items[0].TextEdit.NewText != "local.md" {
				t.Fatalf("completion=%#v", completion)
			}
			definition := request("textDocument/definition", string(params)).(*lang.Location)
			if definition == nil || definition.URI != lang.PathToURI(target) {
				t.Fatalf("definition=%#v", definition)
			}
			hover := request("textDocument/hover", string(params)).(*lang.Hover)
			if hover == nil || !strings.Contains(hover.Contents.Value, target) {
				t.Fatalf("hover=%#v", hover)
			}
			semantic := request("textDocument/semanticTokens/full", string(params)).(map[string]any)["data"].([]int)
			if len(semantic) != 5 || semantic[3] != int(lang.TokenFile) {
				t.Fatalf("semantic=%v", semantic)
			}
		})
	}
}

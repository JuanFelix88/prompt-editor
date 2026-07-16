package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/juanandrade/prompt-cli/internal/lang"
)

const maxMessageBytes = 16 << 20

var errFrameTooLarge = errors.New("LSP frame exceeds 16 MiB limit")

type Server struct {
	service      *lang.Service
	explicitRoot bool
	in           *bufio.Reader
	out          io.Writer
	log          io.Writer
}

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Error   *rpcError       `json:"error"`
}
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func New(root string, explicit bool, in io.Reader, out, logWriter io.Writer) (*Server, error) {
	s, err := lang.NewService(root)
	if err != nil {
		return nil, err
	}
	return &Server{service: s, explicitRoot: explicit, in: bufio.NewReader(in), out: out, log: logWriter}, nil
}

func (s *Server) Run() error {
	for {
		body, err := s.readFrame()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if errors.Is(err, errFrameTooLarge) {
			return err
		}
		if err != nil {
			fmt.Fprintf(s.log, "lsp framing: %v\n", err)
			continue
		}
		var m message
		if err := json.Unmarshal(body, &m); err != nil {
			fmt.Fprintf(s.log, "lsp json: %v\n", err)
			continue
		}
		if m.Method == "exit" {
			return nil
		}
		result, known, handleErr := s.safeHandle(m)
		if len(m.ID) == 0 {
			continue
		}
		if !known {
			if err := s.writeFrame(response{JSONRPC: "2.0", ID: m.ID, Error: &rpcError{Code: -32601, Message: "method not found"}}); err != nil {
				return err
			}
		} else if handleErr != nil {
			fmt.Fprintf(s.log, "lsp %s: %v\n", m.Method, handleErr)
			if err := s.writeFrame(response{JSONRPC: "2.0", ID: m.ID, Error: &rpcError{Code: -32603, Message: "internal error"}}); err != nil {
				return err
			}
		} else if err := s.writeFrame(map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": result}); err != nil {
			return err
		}
	}
}

func (s *Server) readFrame() ([]byte, error) {
	length, headerBytes := -1, 0
	for {
		line, err := s.in.ReadString('\n')
		if err != nil {
			return nil, err
		}
		headerBytes += len(line)
		if len(line) > 8<<10 || headerBytes > 64<<10 {
			return nil, errFrameTooLarge
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[0]), "Content-Length") {
			n, e := strconv.Atoi(strings.TrimSpace(parts[1]))
			if e == nil && n >= 0 {
				length = n
			}
		}
	}
	if length < 0 {
		return nil, errors.New("missing Content-Length")
	}
	if length > maxMessageBytes {
		return nil, errFrameTooLarge
	}
	body := make([]byte, length)
	_, err := io.ReadFull(s.in, body)
	return body, err
}
func (s *Server) writeFrame(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.out, "Content-Length: %d\r\n\r\n%s", len(b), b)
	return err
}

func decode[T any](raw json.RawMessage) (T, error) {
	var v T
	err := json.Unmarshal(raw, &v)
	return v, err
}

func (s *Server) safeHandle(m message) (result any, known bool, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic: %v", recovered)
			known = true
		}
	}()
	return s.handle(m)
}

func (s *Server) handle(m message) (any, bool, error) {
	switch m.Method {
	case "initialize":
		p, err := decode[struct {
			RootURI          string `json:"rootUri"`
			WorkspaceFolders []struct {
				URI string `json:"uri"`
			} `json:"workspaceFolders"`
		}](m.Params)
		if err != nil {
			return nil, true, err
		}
		if !s.explicitRoot {
			uri := p.RootURI
			if uri == "" && len(p.WorkspaceFolders) > 0 {
				uri = p.WorkspaceFolders[0].URI
			}
			if uri != "" {
				path, e := lang.URIToPath(uri)
				if e == nil {
					s.service, e = lang.NewService(path)
					if e != nil {
						return nil, true, e
					}
				}
			}
		}
		return map[string]any{"capabilities": map[string]any{
			"textDocumentSync":   1,
			"completionProvider": map[string]any{"triggerCharacters": []string{"@", "/", "["}, "resolveProvider": false},
			"hoverProvider":      true, "definitionProvider": true,
			"semanticTokensProvider": map[string]any{"legend": map[string]any{"tokenTypes": lang.SemanticLegend, "tokenModifiers": []string{}}, "full": true},
		}, "serverInfo": map[string]string{"name": "prompt-cli", "version": "0.1.0"}}, true, nil
	case "initialized":
		return nil, true, nil
	case "shutdown":
		return nil, true, nil
	case "textDocument/didOpen":
		p, err := decode[struct {
			TextDocument struct {
				URI  string `json:"uri"`
				Text string `json:"text"`
			} `json:"textDocument"`
		}](m.Params)
		if err == nil {
			s.service.Documents.Open(p.TextDocument.URI, p.TextDocument.Text)
		}
		return nil, true, err
	case "textDocument/didChange":
		p, err := decode[struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}](m.Params)
		if err == nil && len(p.ContentChanges) > 0 {
			s.service.Documents.Change(p.TextDocument.URI, p.ContentChanges[0].Text)
		}
		return nil, true, err
	case "textDocument/didClose":
		p, err := decode[struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}](m.Params)
		if err == nil {
			s.service.Documents.Close(p.TextDocument.URI)
		}
		return nil, true, err
	case "textDocument/completion":
		p, err := decode[textPosition](m.Params)
		if err != nil {
			return nil, true, err
		}
		return s.service.Complete(p.TextDocument.URI, p.Position), true, nil
	case "textDocument/hover":
		p, err := decode[textPosition](m.Params)
		if err != nil {
			return nil, true, err
		}
		return s.service.Hover(p.TextDocument.URI, p.Position), true, nil
	case "textDocument/definition":
		p, err := decode[textPosition](m.Params)
		if err != nil {
			return nil, true, err
		}
		return s.service.Definition(p.TextDocument.URI, p.Position), true, nil
	case "textDocument/semanticTokens/full":
		p, err := decode[struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}](m.Params)
		if err != nil {
			return nil, true, err
		}
		return map[string]any{"data": lang.EncodeSemantic(s.service.Semantic(p.TextDocument.URI))}, true, nil
	default:
		return nil, false, nil
	}
}

type textPosition struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position lang.Position `json:"position"`
}

func Run(root string, explicit bool) error {
	s, err := New(root, explicit, os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	return s.Run()
}

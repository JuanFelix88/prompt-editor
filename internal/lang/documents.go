package lang

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type Documents struct {
	mu   sync.RWMutex
	text map[string]string
}

func NewDocuments() *Documents { return &Documents{text: make(map[string]string)} }

func (d *Documents) Open(uri, text string)   { d.set(uri, text) }
func (d *Documents) Change(uri, text string) { d.set(uri, text) }
func (d *Documents) set(uri, text string) {
	d.mu.Lock()
	d.text[uri] = text
	d.mu.Unlock()
}
func (d *Documents) Close(uri string) {
	d.mu.Lock()
	delete(d.text, uri)
	d.mu.Unlock()
}
func (d *Documents) Get(uri string) (string, bool) {
	d.mu.RLock()
	text, ok := d.text[uri]
	d.mu.RUnlock()
	return text, ok
}

func (d *Documents) Text(uri string) string {
	if text, ok := d.Get(uri); ok {
		return text
	}
	path, err := URIToPath(uri)
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func PathToURI(path string) string {
	abs, _ := filepath.Abs(path)
	slash := filepath.ToSlash(abs)
	if runtime.GOOS == "windows" && !strings.HasPrefix(slash, "/") {
		slash = "/" + slash
	}
	return (&url.URL{Scheme: "file", Path: slash}).String()
}

func URIToPath(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" || u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
		return "", errors.New("only local file URIs are supported")
	}
	path, err := url.PathUnescape(u.Path)
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" && len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return filepath.FromSlash(path), nil
}

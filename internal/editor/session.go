package editor

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/juanandrade/prompt-cli/internal/lang"
)

// A session owns one terminal/input loop and suspends editor instances in memory.
// No nested Run calls: competing stdin readers would steal each other's keys.
type editorSession struct {
	current *Editor
	parents []*Editor
	result  string
}

func (s *editorSession) openReference() error {
	e := s.current
	e.service.Documents.Change(e.uri, e.buffer.Text())
	pos := lang.Position{Line: e.buffer.Row, Character: lang.UTF16Len(string(e.buffer.Lines[e.buffer.Row][:e.buffer.Col]))}
	path, ok := e.service.FileReferenceTarget(e.uri, pos)
	if !ok {
		return errors.New("no file reference at cursor")
	}
	// Do not create a second writable buffer for a suspended file: saving it
	// could otherwise be silently undone when its original buffer is saved.
	for _, open := range append(append([]*Editor(nil), s.parents...), e) {
		if !open.anonymous && sameFile(path, open.buffer.Path) {
			return errors.New("file is already open in this session")
		}
	}
	b, err := Load(path)
	if err != nil {
		return err
	}
	child, err := newEditor(b, lang.PathToURI(path), e.service.Root, false)
	if err != nil {
		return err
	}
	child.output = e.output
	child.openedAt = time.Now()
	s.parents = append(s.parents, e)
	s.current = child
	return nil
}

func sameFile(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) || runtime.GOOS == "windows" && strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) {
		return true
	}
	ai, ae := os.Stat(a)
	bi, be := os.Stat(b)
	return ae == nil && be == nil && os.SameFile(ai, bi)
}

func (s *editorSession) handle(k Key) (changed, complete bool, exit int) {
	e := s.current
	if k.Kind == KeyOpenReference && !e.gotoOpen && !e.confirmCancel {
		if err := s.openReference(); err != nil {
			e.status = "open failed: " + err.Error()
		}
		return false, false, 0
	}
	changed, complete, exit = e.handle(k)
	if exit == 1 {
		result, done := e.confirm()
		if !done {
			return changed, complete, 0
		}
		s.result = result
	}
	if exit != 0 && len(s.parents) > 0 {
		s.current = s.parents[len(s.parents)-1]
		s.parents[len(s.parents)-1] = nil
		s.parents = s.parents[:len(s.parents)-1]
		s.result = ""
		// Include edits made just before opening and refresh missing-file colors
		// after the child creates a file, without waiting for the index TTL.
		parent := s.current
		parent.service.Documents.Change(parent.uri, parent.buffer.Text())
		parent.tokens = parent.service.Semantic(parent.uri)
		return false, false, 0
	}
	return
}

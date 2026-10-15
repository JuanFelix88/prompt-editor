package lang

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type IndexEntry struct {
	Path, Insert, Base string
	Directory          bool
	Depth              int
}

type WorkspaceIndex struct {
	root      string
	mu        sync.Mutex
	entries   []IndexEntry
	built     time.Time
	truncated bool
}

var excludedDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true, ".vs": true, ".vscode": true, ".idea": true,
	".next": true, ".nuxt": true, ".svelte-kit": true, ".turbo": true, "bin": true,
	"build": true, "coverage": true, "dist": true, "node_modules": true, "obj": true,
	"out": true, "target": true, "vendor": true,
}

func NewWorkspaceIndex(root string) *WorkspaceIndex { return &WorkspaceIndex{root: root} }

func (w *WorkspaceIndex) Get() ([]IndexEntry, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.entries == nil || time.Since(w.built) >= IndexTTL {
		w.entries, w.truncated = scanWorkspace(w.root)
		w.built = time.Now()
	}
	return append([]IndexEntry(nil), w.entries...), w.truncated
}

func (w *WorkspaceIndex) Warm() { go func() { w.Get() }() }

func scanWorkspace(root string) ([]IndexEntry, bool) {
	type queued struct {
		path, rel string
		depth     int
	}
	queue := []queued{{path: root}}
	out := make([]IndexEntry, 0, 256)
	for len(queue) > 0 {
		q := queue[0]
		queue = queue[1:]
		items, err := os.ReadDir(q.path)
		if err != nil {
			continue
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].IsDir() != items[j].IsDir() {
				return items[i].IsDir()
			}
			return items[i].Name() < items[j].Name()
		})
		for _, item := range items {
			if item.IsDir() && excludedDirs[item.Name()] {
				continue
			}
			rel := item.Name()
			if q.rel != "" {
				rel = q.rel + "/" + item.Name()
			}
			entry := IndexEntry{Path: rel, Insert: rel, Base: item.Name(), Directory: item.IsDir(), Depth: q.depth + 1}
			if entry.Directory {
				entry.Insert += "/"
			}
			out = append(out, entry)
			if len(out) >= MaxIndexEntries {
				return out, true
			}
			if item.IsDir() {
				queue = append(queue, queued{filepath.Join(q.path, item.Name()), rel, q.depth + 1})
			}
		}
	}
	return out, false
}

func normalizeReference(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.TrimPrefix(value, "@")
	value = strings.TrimLeft(value, "/")
	return value
}

func referencePath(base, value string) (string, bool) {
	value = filepath.FromSlash(normalizeReference(value))
	if strings.ContainsRune(value, 0) || filepath.IsAbs(value) || filepath.VolumeName(value) != "" {
		return "", false
	}
	return filepath.Clean(filepath.Join(base, value)), true
}

func Resolve(base, value string) (string, os.FileInfo, bool) {
	target, ok := referencePath(base, value)
	if !ok {
		return "", nil, false
	}
	info, err := os.Stat(target)
	if err != nil {
		return target, nil, false
	}
	return target, info, true
}

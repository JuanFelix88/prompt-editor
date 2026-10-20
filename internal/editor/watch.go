package editor

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// One directory watch follows the active editor. Suspended parents are checked
// when restored. Watching the directory also catches atomic file replacements.
type fileWatch struct {
	*fsnotify.Watcher
	path, directory string
}

func newFileWatch(e *Editor) (*fileWatch, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &fileWatch{Watcher: watcher}
	if err := w.follow(e); err != nil {
		watcher.Close()
		return nil, err
	}
	return w, nil
}

func (w *fileWatch) follow(e *Editor) error {
	w.path = ""
	directory := ""
	if !e.anonymous {
		path, err := filepath.Abs(e.buffer.Path)
		if err != nil {
			return err
		}
		w.path = path
		directory = filepath.Dir(path)
		// New references may have missing parent directories. Watch the closest
		// existing ancestor until creation events let us move the watch closer.
		for {
			info, err := os.Stat(directory)
			if err == nil && info.IsDir() {
				break
			}
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			parent := filepath.Dir(directory)
			if parent == directory {
				return os.ErrNotExist
			}
			directory = parent
		}
	}
	if directory == w.directory {
		return nil
	}
	if directory != "" {
		if err := w.Add(directory); err != nil {
			return err
		}
	}
	if w.directory != "" {
		// Removal can already have happened automatically after a rename/delete.
		_ = w.Remove(w.directory)
	}
	w.directory = directory
	return nil
}

func (w *fileWatch) relevant(event fsnotify.Event) bool {
	if w.path == "" {
		return false
	}
	rel, err := filepath.Rel(event.Name, w.path)
	return err == nil && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

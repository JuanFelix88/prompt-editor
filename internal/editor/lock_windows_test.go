package editor

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestKeepOpenDoesNotLockFile(t *testing.T) {
	if path := os.Getenv("PROMPT_CLI_LOCK_PROBE_PATH"); path != "" {
		// Run in a separate process to catch Windows sharing violations, not
		// merely demonstrate that another Go handle in this process can open it.
		name, err := windows.UTF16PtrFromString(path)
		if err != nil {
			t.Fatal(err)
		}
		handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE,
			0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
		if err != nil {
			t.Fatalf("exclusive open (no sharing): %v", err)
		}
		file := os.NewFile(uintptr(handle), path)
		if err := file.Truncate(0); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if _, err := file.WriteString("exclusive external write"); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(path, path+".moved"); err != nil {
			t.Fatalf("rename: %v", err)
		}
		if err := os.Rename(path+".moved", path); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path+".replacement", []byte("replacement"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(path+".replacement", path); err != nil {
			t.Fatalf("atomic replacement: %v", err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if err := os.WriteFile(path, []byte(os.Getenv("PROMPT_CLI_LOCK_PROBE_TEXT")), 0644); err != nil {
			t.Fatalf("recreate: %v", err)
		}
		return
	}

	for _, state := range []string{"clean", "saved", "unsaved"} {
		t.Run(state, func(t *testing.T) {
			e := externalEditor(t, "original")
			e.handle(Key{Kind: KeyToggleKeepOpen})
			testFileWatch(t, e) // Keep the real native watcher alive throughout.
			if state != "clean" {
				e.buffer.InsertText("local ")
			}
			if state == "saved" {
				if _, _, exit := e.handle(Key{Kind: KeySave}); exit != 0 || e.buffer.Dirty {
					t.Fatal("Keep open save failed or requested exit")
				}
			}
			localText := e.buffer.Text()
			externalText := "external " + state
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestKeepOpenDoesNotLockFile$")
			cmd.Env = append(os.Environ(), "PROMPT_CLI_LOCK_PROBE_PATH="+e.buffer.Path,
				"PROMPT_CLI_LOCK_PROBE_TEXT="+externalText)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("external process failed: %v\n%s", err, output)
			}
			e.checkExternal()
			if !e.keepOpen {
				t.Fatal("Keep open was disabled")
			}
			if state == "unsaved" {
				if e.buffer.Text() != localText || !strings.Contains(e.externalStatus, "changed externally") {
					t.Fatal("local edits lost or conflict warning missing")
				}
			} else if e.buffer.Text() != externalText {
				t.Fatal("external change was not reloaded")
			}
		})
	}
}

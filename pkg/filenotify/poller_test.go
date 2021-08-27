package filenotify // import "github.com/docker/docker/pkg/filenotify"

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestPollerAddRemove(t *testing.T) {
	w := NewPollingWatcher()

	if err := w.Add("no-such-file"); err == nil {
		t.Fatal("should have gotten error when adding a non-existent file")
	}
	if err := w.Remove("no-such-file"); err == nil {
		t.Fatal("should have gotten error when removing non-existent watch")
	}

	f, err := os.CreateTemp("", "asdf")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(f.Name())

	if err := w.Add(f.Name()); err != nil {
		t.Fatal(err)
	}

	if err := w.Remove(f.Name()); err != nil {
		t.Fatal(err)
	}
}

func TestPollerEvent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("No chmod on Windows")
	}
	w := NewPollingWatcher()

	f, err := os.CreateTemp("", "test-poller")
	if err != nil {
		t.Fatal("error creating temp file")
	}
	defer os.RemoveAll(f.Name())
	f.Close()

	if err := w.Add(f.Name()); err != nil {
		t.Fatal(err)
	}

	select {
	case <-w.Events():
		t.Fatal("got event before anything happened")
	case <-w.Errors():
		t.Fatal("got error before anything happened")
	default:
	}

	if err := os.WriteFile(f.Name(), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	assertFileMode(t, f.Name(), 0600)
	if err := assertEvent(w, fsnotify.Write); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(f.Name(), 0644); err != nil {
		t.Fatal(err)
	}
	assertFileMode(t, f.Name(), 0644)
	if err := assertEvent(w, fsnotify.Chmod); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(f.Name()); err != nil {
		t.Fatal(err)
	}
	if err := assertEvent(w, fsnotify.Remove); err != nil {
		t.Fatal(err)
	}
}

func TestPollerClose(t *testing.T) {
	w := NewPollingWatcher()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	// test double-close
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	f, err := os.CreateTemp("", "asdf")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(f.Name())
	if err := w.Add(f.Name()); err == nil {
		t.Fatal("should have gotten error adding watch for closed watcher")
	}
}

func assertFileMode(t *testing.T, fileName string, mode uint32) {
	t.Helper()
	f, err := os.Stat(fileName)
	if err != nil {
		t.Fatal(err)
	}
	if f.Mode() != os.FileMode(mode) {
		t.Fatalf("expected file %s to have mode %#o, but got %#o", fileName, mode, f.Mode())
	}
}

func assertEvent(w FileWatcher, eType fsnotify.Op) error {
	var err error
	select {
	case e := <-w.Events():
		if e.Op != eType {
			err = fmt.Errorf("got wrong event type, expected %q: %v", eType, e.Op)
		}
	case e := <-w.Errors():
		err = fmt.Errorf("got unexpected error waiting for events %v: %v", eType, e)
	case <-time.After(watchWaitTime * 3):
		err = fmt.Errorf("timeout waiting for event %v", eType)
	}
	return err
}

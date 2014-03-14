package lock

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestLock(t *testing.T) {
	td, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(td)

	path := filepath.Join(td, "foo.lock")

	l, c, err := Lock(path)
	if err != nil {
		t.Fatalf("Cannot lock file: %v\n", err)
	}
	if c != nil {
		t.Fatalf("Expected empty channel as the file is not locked")
	}

	w, c, err := Lock(path)
	if err == nil {
		t.Fatalf("Expected error 'file already locked' but got none\n")
	}
	if err != ErrAlreadyLocked {
		t.Fatalf("Expected error 'file already locked' but got %v\n", err)
	}
	if c == nil {
		t.Fatalf("Expected non-empty channel as the file is locked")
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Unexecpected close error: %v\n", err)
	}

	if err := l.Close(); err != nil {
		t.Fatalf("Cannot unlock file: %v\n", err)
	}
}

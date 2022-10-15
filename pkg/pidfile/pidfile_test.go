package pidfile // import "github.com/docker/docker/pkg/pidfile"

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testfile")

	err := Write(path, 0)
	if err == nil {
		t.Fatal("writing PID < 1 should fail")
	}

	err = Write(path, os.Getpid())
	if err != nil {
		t.Fatal("Could not create test file", err)
	}

	err = Write(path, os.Getpid())
	if err == nil {
		t.Fatal("Test file creation not blocked")
	}
}

package pidfile // import "github.com/docker/docker/pkg/pidfile"

import (
	"path/filepath"
	"testing"
)

func TestWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testfile")
	err := Write(path)
	if err != nil {
		t.Fatal("Could not create test file", err)
	}

	err = Write(path)
	if err == nil {
		t.Fatal("Test file creation not blocked")
	}
}

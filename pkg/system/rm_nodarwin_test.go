//go:build !darwin
// +build !darwin

package system // import "github.com/docker/docker/pkg/system"

import (
	"os"
	"testing"
)

func TestEnsureRemoveAllNotExist(t *testing.T) {
	// should never return an error for a non-existent path
	if err := EnsureRemoveAll("/non/existent/path"); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureRemoveAllWithDir(t *testing.T) {
	dir, err := os.MkdirTemp("", "test-ensure-removeall-with-dir")
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureRemoveAll(dir); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureRemoveAllWithFile(t *testing.T) {
	tmp, err := os.CreateTemp("", "test-ensure-removeall-with-dir")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	if err := EnsureRemoveAll(tmp.Name()); err != nil {
		t.Fatal(err)
	}
}

package plugin // import "github.com/docker/docker/plugin"

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicRemoveAllNormal(t *testing.T) {
	dir, err := os.MkdirTemp("", "atomic-remove-with-normal")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir) // just try to make sure this gets cleaned up

	if err := atomicRemoveAll(dir); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("dir should be gone: %v", err)
	}
	if _, err := os.Stat(dir + "-removing"); !os.IsNotExist(err) {
		t.Fatalf("dir should be gone: %v", err)
	}
}

func TestAtomicRemoveAllAlreadyExists(t *testing.T) {
	dir, err := os.MkdirTemp("", "atomic-remove-already-exists")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir) // just try to make sure this gets cleaned up

	if err := os.MkdirAll(dir+"-removing", 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir + "-removing")

	if err := atomicRemoveAll(dir); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("dir should be gone: %v", err)
	}
	if _, err := os.Stat(dir + "-removing"); !os.IsNotExist(err) {
		t.Fatalf("dir should be gone: %v", err)
	}
}

func TestAtomicRemoveAllNotExist(t *testing.T) {
	if err := atomicRemoveAll("/not-exist"); err != nil {
		t.Fatal(err)
	}

	dir, err := os.MkdirTemp("", "atomic-remove-already-exists")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir) // just try to make sure this gets cleaned up

	// create the removing dir, but not the "real" one
	foo := filepath.Join(dir, "foo")
	removing := dir + "-removing"
	if err := os.MkdirAll(removing, 0755); err != nil {
		t.Fatal(err)
	}

	if err := atomicRemoveAll(dir); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(foo); !os.IsNotExist(err) {
		t.Fatalf("dir should be gone: %v", err)
	}
	if _, err := os.Stat(removing); !os.IsNotExist(err) {
		t.Fatalf("dir should be gone: %v", err)
	}
}

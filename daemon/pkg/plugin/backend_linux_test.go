package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicRemoveAllNormal(t *testing.T) {
	dir := t.TempDir()

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
	dir := t.TempDir()

	if err := os.MkdirAll(dir+"-removing", 0o755); err != nil {
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

	dir := t.TempDir()

	// create the removing dir, but not the "real" one
	foo := filepath.Join(dir, "foo")
	removing := dir + "-removing"
	if err := os.MkdirAll(removing, 0o755); err != nil {
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

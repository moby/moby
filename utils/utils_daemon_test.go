package utils

import (
	"os"
	"path"
	"testing"
)

func TestIsFileOwner(t *testing.T) {
	var err error
	var file *os.File

	if file, err = os.Create(path.Join(os.TempDir(), "testIsFileOwner")); err != nil {
		t.Fatalf("failed to create file: %s", err)
	}
	file.Close()

	if ok := IsFileOwner(path.Join(os.TempDir(), "testIsFileOwner")); !ok {
		t.Fatalf("User should be owner of file")
	}

	if err = os.Remove(path.Join(os.TempDir(), "testIsFileOwner")); err != nil {
		t.Fatalf("failed to remove file: %s", err)
	}

}

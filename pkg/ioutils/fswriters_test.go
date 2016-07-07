package ioutils

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteToFile(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "atomic-writers-test")
	if err != nil {
		t.Fatalf("Error when creating temporary directory: %s", err)
	}
	defer os.RemoveAll(tmpDir)

	expected := []byte("barbaz")
	if err := AtomicWriteFile(filepath.Join(tmpDir, "foo"), expected, 0666); err != nil {
		t.Fatalf("Error writing to file: %v", err)
	}

	actual, err := ioutil.ReadFile(filepath.Join(tmpDir, "foo"))
	if err != nil {
		t.Fatalf("Error reading from file: %v", err)
	}

	if bytes.Compare(actual, expected) != 0 {
		t.Fatalf("Data mismatch, expected %q, got %q", expected, actual)
	}

	st, err := os.Stat(filepath.Join(tmpDir, "foo"))
	if err != nil {
		t.Fatalf("Error statting file: %v", err)
	}
	if expected := os.FileMode(0666); st.Mode() != expected {
		t.Fatalf("Mode mismatched, expected %o, got %o", expected, st.Mode())
	}
}

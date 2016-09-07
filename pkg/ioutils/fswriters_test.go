package ioutils

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestAtomicWriteToFile(c *check.C) {
	tmpDir, err := ioutil.TempDir("", "atomic-writers-test")
	if err != nil {
		c.Fatalf("Error when creating temporary directory: %s", err)
	}
	defer os.RemoveAll(tmpDir)

	expected := []byte("barbaz")
	if err := AtomicWriteFile(filepath.Join(tmpDir, "foo"), expected, 0666); err != nil {
		c.Fatalf("Error writing to file: %v", err)
	}

	actual, err := ioutil.ReadFile(filepath.Join(tmpDir, "foo"))
	if err != nil {
		c.Fatalf("Error reading from file: %v", err)
	}

	if bytes.Compare(actual, expected) != 0 {
		c.Fatalf("Data mismatch, expected %q, got %q", expected, actual)
	}

	st, err := os.Stat(filepath.Join(tmpDir, "foo"))
	if err != nil {
		c.Fatalf("Error statting file: %v", err)
	}
	if expected := os.FileMode(0666); st.Mode() != expected {
		c.Fatalf("Mode mismatched, expected %o, got %o", expected, st.Mode())
	}
}

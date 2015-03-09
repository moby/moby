package utils

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestReplaceAndAppendEnvVars(t *testing.T) {
	var (
		d = []string{"HOME=/"}
		o = []string{"HOME=/root", "TERM=xterm"}
	)

	env := ReplaceOrAppendEnvValues(d, o)
	if len(env) != 2 {
		t.Fatalf("expected len of 2 got %d", len(env))
	}
	if env[0] != "HOME=/root" {
		t.Fatalf("expected HOME=/root got '%s'", env[0])
	}
	if env[1] != "TERM=xterm" {
		t.Fatalf("expected TERM=xterm got '%s'", env[1])
	}
}

// Reading a symlink to a directory must return the directory
func TestReadSymlinkedDirectoryExistingDirectory(t *testing.T) {
	var err error
	if err = os.Mkdir("/tmp/testReadSymlinkToExistingDirectory", 0777); err != nil {
		t.Errorf("failed to create directory: %s", err)
	}

	if err = os.Symlink("/tmp/testReadSymlinkToExistingDirectory", "/tmp/dirLinkTest"); err != nil {
		t.Errorf("failed to create symlink: %s", err)
	}

	var path string
	if path, err = ReadSymlinkedDirectory("/tmp/dirLinkTest"); err != nil {
		t.Fatalf("failed to read symlink to directory: %s", err)
	}

	if path != "/tmp/testReadSymlinkToExistingDirectory" {
		t.Fatalf("symlink returned unexpected directory: %s", path)
	}

	if err = os.Remove("/tmp/testReadSymlinkToExistingDirectory"); err != nil {
		t.Errorf("failed to remove temporary directory: %s", err)
	}

	if err = os.Remove("/tmp/dirLinkTest"); err != nil {
		t.Errorf("failed to remove symlink: %s", err)
	}
}

// Reading a non-existing symlink must fail
func TestReadSymlinkedDirectoryNonExistingSymlink(t *testing.T) {
	var path string
	var err error
	if path, err = ReadSymlinkedDirectory("/tmp/test/foo/Non/ExistingPath"); err == nil {
		t.Fatalf("error expected for non-existing symlink")
	}

	if path != "" {
		t.Fatalf("expected empty path, but '%s' was returned", path)
	}
}

// Reading a symlink to a file must fail
func TestReadSymlinkedDirectoryToFile(t *testing.T) {
	var err error
	var file *os.File

	if file, err = os.Create("/tmp/testReadSymlinkToFile"); err != nil {
		t.Fatalf("failed to create file: %s", err)
	}

	file.Close()

	if err = os.Symlink("/tmp/testReadSymlinkToFile", "/tmp/fileLinkTest"); err != nil {
		t.Errorf("failed to create symlink: %s", err)
	}

	var path string
	if path, err = ReadSymlinkedDirectory("/tmp/fileLinkTest"); err == nil {
		t.Fatalf("ReadSymlinkedDirectory on a symlink to a file should've failed")
	}

	if path != "" {
		t.Fatalf("path should've been empty: %s", path)
	}

	if err = os.Remove("/tmp/testReadSymlinkToFile"); err != nil {
		t.Errorf("failed to remove file: %s", err)
	}

	if err = os.Remove("/tmp/fileLinkTest"); err != nil {
		t.Errorf("failed to remove symlink: %s", err)
	}
}

func TestWriteCounter(t *testing.T) {
	dummy1 := "This is a dummy string."
	dummy2 := "This is another dummy string."
	totalLength := int64(len(dummy1) + len(dummy2))

	reader1 := strings.NewReader(dummy1)
	reader2 := strings.NewReader(dummy2)

	var buffer bytes.Buffer
	wc := NewWriteCounter(&buffer)

	reader1.WriteTo(wc)
	reader2.WriteTo(wc)

	if wc.Count != totalLength {
		t.Errorf("Wrong count: %d vs. %d", wc.Count, totalLength)
	}

	if buffer.String() != dummy1+dummy2 {
		t.Error("Wrong message written")
	}
}

func TestImageReference(t *testing.T) {
	tests := []struct {
		repo     string
		ref      string
		expected string
	}{
		{"repo", "tag", "repo:tag"},
		{"repo", "sha256:c100b11b25d0cacd52c14e0e7bf525e1a4c0e6aec8827ae007055545909d1a64", "repo@sha256:c100b11b25d0cacd52c14e0e7bf525e1a4c0e6aec8827ae007055545909d1a64"},
	}

	for i, test := range tests {
		actual := ImageReference(test.repo, test.ref)
		if test.expected != actual {
			t.Errorf("%d: expected %q, got %q", i, test.expected, actual)
		}
	}
}

func TestDigestReference(t *testing.T) {
	input := "sha256:c100b11b25d0cacd52c14e0e7bf525e1a4c0e6aec8827ae007055545909d1a64"
	if !DigestReference(input) {
		t.Errorf("Expected DigestReference=true for input %q", input)
	}

	input = "latest"
	if DigestReference(input) {
		t.Errorf("Unexpected DigestReference=true for input %q", input)
	}
}

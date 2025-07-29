package fileutils

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Reading a symlink to a directory must return the directory
func TestReadSymlinkedDirectoryExistingDirectory(t *testing.T) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		t.Skip("Needs porting to Windows")
	}

	// On macOS, tmp itself is symlinked, so resolve this one upfront;
	// see https://github.com/golang/go/issues/56259
	tmpDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	srcPath := filepath.Join(tmpDir, "/testReadSymlinkToExistingDirectory")
	dstPath := filepath.Join(tmpDir, "/dirLinkTest")
	if err = os.Mkdir(srcPath, 0o777); err != nil {
		t.Errorf("failed to create directory: %s", err)
	}

	if err = os.Symlink(srcPath, dstPath); err != nil {
		t.Errorf("failed to create symlink: %s", err)
	}

	var symlinkedPath string
	if symlinkedPath, err = ReadSymlinkedDirectory(dstPath); err != nil {
		t.Fatalf("failed to read symlink to directory: %s", err)
	}

	if symlinkedPath != srcPath {
		t.Fatalf("symlink returned unexpected directory: %s", symlinkedPath)
	}

	if err = os.Remove(srcPath); err != nil {
		t.Errorf("failed to remove temporary directory: %s", err)
	}

	if err = os.Remove(dstPath); err != nil {
		t.Errorf("failed to remove symlink: %s", err)
	}
}

// Reading a non-existing symlink must fail
func TestReadSymlinkedDirectoryNonExistingSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	symLinkedPath, err := ReadSymlinkedDirectory(path.Join(tmpDir, "/Non/ExistingPath"))
	if err == nil {
		t.Errorf("error expected for non-existing symlink")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Expected an os.ErrNotExist, got: %v", err)
	}
	if symLinkedPath != "" {
		t.Fatalf("expected empty path, but '%s' was returned", symLinkedPath)
	}
}

// Reading a symlink to a file must fail
func TestReadSymlinkedDirectoryToFile(t *testing.T) {
	// TODO Windows: Port this test
	if runtime.GOOS == "windows" {
		t.Skip("Needs porting to Windows")
	}
	var err error
	var file *os.File

	// #nosec G303
	if file, err = os.Create("/tmp/testReadSymlinkToFile"); err != nil {
		t.Fatalf("failed to create file: %s", err)
	}

	_ = file.Close()

	if err = os.Symlink("/tmp/testReadSymlinkToFile", "/tmp/fileLinkTest"); err != nil {
		t.Errorf("failed to create symlink: %s", err)
	}

	symlinkedPath, err := ReadSymlinkedDirectory("/tmp/fileLinkTest")
	if err == nil {
		t.Errorf("ReadSymlinkedDirectory on a symlink to a file should've failed")
	}
	if !strings.HasPrefix(err.Error(), "canonical path points to a file") {
		t.Errorf("unexpected error: %v", err)
	}

	if symlinkedPath != "" {
		t.Errorf("path should've been empty: %s", symlinkedPath)
	}

	if err = os.Remove("/tmp/testReadSymlinkToFile"); err != nil {
		t.Errorf("failed to remove file: %s", err)
	}

	if err = os.Remove("/tmp/fileLinkTest"); err != nil {
		t.Errorf("failed to remove symlink: %s", err)
	}
}

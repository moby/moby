package fileutils // import "github.com/docker/docker/pkg/fileutils"

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// CopyFile with invalid src
func TestCopyFileWithInvalidSrc(t *testing.T) {
	tempDir := t.TempDir()
	bytes, err := CopyFile(filepath.Join(tempDir, "/invalid/file/path"), path.Join(t.TempDir(), "dest"))
	if err == nil {
		t.Error("Should have fail to copy an invalid src file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Expected an os.ErrNotExist, got: %v", err)
	}
	if bytes != 0 {
		t.Errorf("Should have written 0 bytes, got: %d", bytes)
	}
}

// CopyFile with invalid dest
func TestCopyFileWithInvalidDest(t *testing.T) {
	tempFolder := t.TempDir()
	src := path.Join(tempFolder, "file")
	err := os.WriteFile(src, []byte("content"), 0o740)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := CopyFile(src, path.Join(tempFolder, "/invalid/dest/path"))
	if err == nil {
		t.Error("Should have fail to copy an invalid src file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Expected an os.ErrNotExist, got: %v", err)
	}
	if bytes != 0 {
		t.Errorf("Should have written 0 bytes, got: %d", bytes)
	}
}

// CopyFile with same src and dest
func TestCopyFileWithSameSrcAndDest(t *testing.T) {
	file := path.Join(t.TempDir(), "file")
	err := os.WriteFile(file, []byte("content"), 0o740)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := CopyFile(file, file)
	if err != nil {
		t.Fatal(err)
	}
	if bytes != 0 {
		t.Fatal("Should have written 0 bytes as it is the same file.")
	}
}

// CopyFile with same src and dest but path is different and not clean
func TestCopyFileWithSameSrcAndDestWithPathNameDifferent(t *testing.T) {
	testFolder := path.Join(t.TempDir(), "test")
	err := os.Mkdir(testFolder, 0o740)
	if err != nil {
		t.Fatal(err)
	}
	file := path.Join(testFolder, "file")
	sameFile := testFolder + "/../test/file"
	err = os.WriteFile(file, []byte("content"), 0o740)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := CopyFile(file, sameFile)
	if err != nil {
		t.Fatal(err)
	}
	if bytes != 0 {
		t.Fatal("Should have written 0 bytes as it is the same file.")
	}
}

func TestCopyFile(t *testing.T) {
	tempFolder := t.TempDir()
	src := path.Join(tempFolder, "src")
	dest := path.Join(tempFolder, "dest")
	err := os.WriteFile(src, []byte("content"), 0o777)
	if err != nil {
		t.Error(err)
	}
	err = os.WriteFile(dest, []byte("destContent"), 0o777)
	if err != nil {
		t.Error(err)
	}
	bytes, err := CopyFile(src, dest)
	if err != nil {
		t.Fatal(err)
	}
	if bytes != 7 {
		t.Fatalf("Should have written %d bytes but wrote %d", 7, bytes)
	}
	actual, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != "content" {
		t.Fatalf("Dest content was '%s', expected '%s'", string(actual), "content")
	}
}

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

func TestCreateIfNotExistsDir(t *testing.T) {
	folderToCreate := filepath.Join(t.TempDir(), "tocreate")

	if err := CreateIfNotExists(folderToCreate, true); err != nil {
		t.Fatal(err)
	}
	fileinfo, err := os.Stat(folderToCreate)
	if err != nil {
		t.Fatalf("Should have create a folder, got %v", err)
	}

	if !fileinfo.IsDir() {
		t.Errorf("Should have been a dir, seems it's not")
	}
}

func TestCreateIfNotExistsFile(t *testing.T) {
	fileToCreate := filepath.Join(t.TempDir(), "file/to/create")

	if err := CreateIfNotExists(fileToCreate, false); err != nil {
		t.Error(err)
	}
	fileinfo, err := os.Stat(fileToCreate)
	if err != nil {
		t.Fatalf("Should have create a file, got %v", err)
	}

	if fileinfo.IsDir() {
		t.Errorf("Should have been a file, seems it's not")
	}
}

func BenchmarkGetTotalUsedFds(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = GetTotalUsedFds(ctx)
	}
}

package directory

import (
	"os"
	"path/filepath"
	"testing"
)

var (
	tmp = filepath.Join(os.TempDir(), "directory-tests")
)

// Size of an empty directory should be 0
func TestSizeEmpty(t *testing.T) {
	defer os.RemoveAll(tmp)
	dir := filepath.Join(tmp, "testSizeEmptyDirectory")

	if err := os.MkdirAll(dir, 0777); err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}

	if size, _ := Size(dir); size != 0 {
		t.Fatalf("empty directory has size: %d", size)
	}
}

// Size of a directory with one empty file should be 0
func TestSizeEmptyFile(t *testing.T) {
	defer os.RemoveAll(tmp)
	dir := filepath.Join(tmp, "testSizeEmptyFile")

	if err := os.MkdirAll(dir, 0777); err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}

	if _, err := os.Create(filepath.Join(dir, "file")); err != nil {
		t.Fatalf("failed to create file: %s", err)
	}

	if size, _ := Size(dir); size != 0 {
		t.Fatalf("directory with one file has size: %d", size)
	}
}

// Size of a directory with one 5-byte file should be 5
func TestSizeNonemptyFile(t *testing.T) {
	defer os.RemoveAll(tmp)
	dir := filepath.Join(tmp, "testSizeNonemptyFile")

	var err error
	if err = os.MkdirAll(dir, 0777); err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}

	var file *os.File
	if file, err = os.Create(filepath.Join(dir, "file")); err != nil {
		t.Fatalf("failed to create file: %s", err)
	}

	d := []byte{97, 98, 99, 100, 101}
	file.Write(d)

	if size, _ := Size(dir); size != 5 {
		t.Fatalf("directory with one 5-byte file has size: %d", size)
	}
}

// Size of a directory with one empty directory should be 0
func TestSizeNestedDirectoryEmpty(t *testing.T) {
	defer os.RemoveAll(tmp)
	dir := filepath.Join(tmp, "testSizeNestedDirectoryEmpty")

	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0777); err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}

	if size, _ := Size(dir); size != 0 {
		t.Fatalf("directory with one empty directory has size: %d", size)
	}
}

// Test directory with 1 file and 1 empty directory
func TestSizeFileAndNestedDirectoryEmpty(t *testing.T) {
	defer os.RemoveAll(tmp)
	dir := filepath.Join(tmp, "testSizeFileAndNestedDirectoryEmpty")

	var err error
	if err = os.MkdirAll(filepath.Join(dir, "nested"), 0777); err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}

	var file *os.File
	if file, err = os.Create(filepath.Join(dir, "file")); err != nil {
		t.Fatalf("failed to create file: %s", err)
	}

	d := []byte{100, 111, 99, 107, 101, 114}
	file.Write(d)

	if size, _ := Size(dir); size != 6 {
		t.Fatalf("directory with 6-byte file and empty directory has size: %d", size)
	}
}

// Test directory with 1 file and 1 non-empty directory
func TestSizeFileAndNestedDirectoryNonempty(t *testing.T) {
	defer os.RemoveAll(tmp)
	dir := filepath.Join(tmp, "testSizeFileAndNestedDirectoryNonEmpty")

	var err error
	if err = os.MkdirAll(filepath.Join(dir, "nested"), 0777); err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}

	var file *os.File
	if file, err = os.Create(filepath.Join(dir, "file")); err != nil {
		t.Fatalf("failed to create file: %s", err)
	}

	data := []byte{100, 111, 99, 107, 101, 114}
	file.Write(data)

	var nestedFile *os.File
	if nestedFile, err = os.Create(filepath.Join(dir, "nested", "file")); err != nil {
		t.Fatalf("failed to create file: %s", err)
	}

	nestedData := []byte{100, 111, 99, 107, 101, 114}
	nestedFile.Write(nestedData)

	if size, _ := Size(dir); size != 12 {
		t.Fatalf("directory with 6-byte file and empty directory has size: %d", size)
	}
}

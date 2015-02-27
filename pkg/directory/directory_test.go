package directory

import (
	"os"
	"testing"
)

// Size of an empty directory should be 0
func TestSizeEmpty(t *testing.T) {
	var err error
	if err = os.Mkdir("/tmp/testSizeEmptyDirectory", 0777); err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}

	var size int64
	if size, _ = Size("/tmp/testSizeEmptyDirectory"); size != 0 {
		t.Fatalf("empty directory has size: %d", size)
	}
}

// Size of a directory with one empty file should be 0
func TestSizeEmptyFile(t *testing.T) {
	var err error
	if err = os.Mkdir("/tmp/testSizeEmptyFile", 0777); err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}

	if _, err = os.Create("/tmp/testSizeEmptyFile/file"); err != nil {
		t.Fatalf("failed to create file: %s", err)
	}

	var size int64
	if size, _ = Size("/tmp/testSizeEmptyFile"); size != 0 {
		t.Fatalf("directory with one file has size: %d", size)
	}
}

// Size of a directory with one 5-byte file should be 5
func TestSizeNonemptyFile(t *testing.T) {
	var err error
	if err = os.Mkdir("/tmp/testSizeNonemptyFile", 0777); err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}

	var file *os.File
	if file, err = os.Create("/tmp/testSizeNonemptyFile/file"); err != nil {
		t.Fatalf("failed to create file: %s", err)
	}

	d := []byte{97, 98, 99, 100, 101}
	file.Write(d)

	var size int64
	if size, _ = Size("/tmp/testSizeNonemptyFile"); size != 5 {
		t.Fatalf("directory with one 5-byte file has size: %d", size)
	}
}

// Size of a directory with one empty directory should be 0
func TestSizeNestedDirectoryEmpty(t *testing.T) {
	var err error
	if err = os.MkdirAll("/tmp/testSizeNestedDirectoryEmpty/nested", 0777); err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}

	var size int64
	if size, _ = Size("/tmp/testSizeNestedDirectoryEmpty"); size != 0 {
		t.Fatalf("directory with one empty directory has size: %d", size)
	}
}

// Test directory with 1 file and 1 empty directory
func TestSizeFileAndNestedDirectoryEmpty(t *testing.T) {
	var err error
	if err = os.MkdirAll("/tmp/testSizeFileAndNestedDirectoryEmpty/nested", 0777); err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}

	var file *os.File
	if file, err = os.Create("/tmp/testSizeFileAndNestedDirectoryEmpty/file"); err != nil {
		t.Fatalf("failed to create file: %s", err)
	}

	d := []byte{100, 111, 99, 107, 101, 114}
	file.Write(d)

	var size int64
	if size, _ = Size("/tmp/testSizeFileAndNestedDirectoryEmpty"); size != 6 {
		t.Fatalf("directory with 6-byte file and empty directory has size: %d", size)
	}
}

// Test directory with 1 file and 1 non-empty directory
func TestSizeFileAndNestedDirectoryNonempty(t *testing.T) {
	var err error
	if err = os.MkdirAll("/tmp/testSizeFileAndNestedDirectoryEmpty/nested", 0777); err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}

	var file *os.File
	if file, err = os.Create("/tmp/testSizeFileAndNestedDirectoryEmpty/file"); err != nil {
		t.Fatalf("failed to create file: %s", err)
	}

	data := []byte{100, 111, 99, 107, 101, 114}
	file.Write(data)

	var nestedFile *os.File
	if nestedFile, err = os.Create("/tmp/testSizeFileAndNestedDirectoryEmpty/nested/file"); err != nil {
		t.Fatalf("failed to create file: %s", err)
	}

	nestedData := []byte{100, 111, 99, 107, 101, 114}
	nestedFile.Write(nestedData)

	var size int64
	if size, _ = Size("/tmp/testSizeFileAndNestedDirectoryEmpty"); size != 12 {
		t.Fatalf("directory with 6-byte file and empty directory has size: %d", size)
	}
}

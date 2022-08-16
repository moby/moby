package directory // import "github.com/docker/docker/pkg/directory"

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// Size of an empty directory should be 0
func TestSizeEmpty(t *testing.T) {
	var dir string
	var err error
	if dir, err = os.MkdirTemp(os.TempDir(), "testSizeEmptyDirectory"); err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}

	var size int64
	if size, _ = Size(context.Background(), dir); size != 0 {
		t.Fatalf("empty directory has size: %d", size)
	}
}

// Size of a directory with one empty file should be 0
func TestSizeEmptyFile(t *testing.T) {
	var dir string
	var err error
	if dir, err = os.MkdirTemp(os.TempDir(), "testSizeEmptyFile"); err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}

	var file *os.File
	if file, err = os.CreateTemp(dir, "file"); err != nil {
		t.Fatalf("failed to create file: %s", err)
	}
	defer file.Close()

	var size int64
	if size, _ = Size(context.Background(), file.Name()); size != 0 {
		t.Fatalf("directory with one file has size: %d", size)
	}
}

// Size of a directory with one 5-byte file should be 5
func TestSizeNonemptyFile(t *testing.T) {
	var dir string
	var err error
	if dir, err = os.MkdirTemp(os.TempDir(), "testSizeNonemptyFile"); err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}

	var file *os.File
	if file, err = os.CreateTemp(dir, "file"); err != nil {
		t.Fatalf("failed to create file: %s", err)
	}
	defer file.Close()

	d := []byte{97, 98, 99, 100, 101}
	file.Write(d)

	var size int64
	if size, _ = Size(context.Background(), file.Name()); size != 5 {
		t.Fatalf("directory with one 5-byte file has size: %d", size)
	}
}

// Size of a directory with one empty directory should be 0
func TestSizeNestedDirectoryEmpty(t *testing.T) {
	var dir string
	var err error
	if dir, err = os.MkdirTemp(os.TempDir(), "testSizeNestedDirectoryEmpty"); err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}
	if dir, err = os.MkdirTemp(dir, "nested"); err != nil {
		t.Fatalf("failed to create nested directory: %s", err)
	}

	var size int64
	if size, _ = Size(context.Background(), dir); size != 0 {
		t.Fatalf("directory with one empty directory has size: %d", size)
	}
}

// Test directory with 1 file and 1 empty directory
func TestSizeFileAndNestedDirectoryEmpty(t *testing.T) {
	var dir string
	var err error
	if dir, err = os.MkdirTemp(os.TempDir(), "testSizeFileAndNestedDirectoryEmpty"); err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}
	if dir, err = os.MkdirTemp(dir, "nested"); err != nil {
		t.Fatalf("failed to create nested directory: %s", err)
	}

	var file *os.File
	if file, err = os.CreateTemp(dir, "file"); err != nil {
		t.Fatalf("failed to create file: %s", err)
	}
	defer file.Close()

	d := []byte{100, 111, 99, 107, 101, 114}
	file.Write(d)

	var size int64
	if size, _ = Size(context.Background(), dir); size != 6 {
		t.Fatalf("directory with 6-byte file and empty directory has size: %d", size)
	}
}

// Test directory with 1 file and 1 non-empty directory
func TestSizeFileAndNestedDirectoryNonempty(t *testing.T) {
	var dir, dirNested string
	var err error
	if dir, err = os.MkdirTemp(os.TempDir(), "TestSizeFileAndNestedDirectoryNonempty"); err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}
	if dirNested, err = os.MkdirTemp(dir, "nested"); err != nil {
		t.Fatalf("failed to create nested directory: %s", err)
	}

	var file *os.File
	if file, err = os.CreateTemp(dir, "file"); err != nil {
		t.Fatalf("failed to create file: %s", err)
	}
	defer file.Close()

	data := []byte{100, 111, 99, 107, 101, 114}
	file.Write(data)

	var nestedFile *os.File
	if nestedFile, err = os.CreateTemp(dirNested, "file"); err != nil {
		t.Fatalf("failed to create file in nested directory: %s", err)
	}
	defer nestedFile.Close()

	nestedData := []byte{100, 111, 99, 107, 101, 114}
	nestedFile.Write(nestedData)

	var size int64
	if size, _ = Size(context.Background(), dir); size != 12 {
		t.Fatalf("directory with 6-byte file and nested directory with 6-byte file has size: %d", size)
	}
}

// Test migration of directory to a subdir underneath itself
func TestMoveToSubdir(t *testing.T) {
	var outerDir, subDir string
	var err error

	if outerDir, err = os.MkdirTemp(os.TempDir(), "TestMoveToSubdir"); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	if subDir, err = os.MkdirTemp(outerDir, "testSub"); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	// write 4 temp files in the outer dir to get moved
	filesList := []string{"a", "b", "c", "d"}
	for _, fName := range filesList {
		if file, err := os.Create(filepath.Join(outerDir, fName)); err != nil {
			t.Fatalf("couldn't create temp file %q: %v", fName, err)
		} else {
			file.WriteString(fName)
			file.Close()
		}
	}

	if err = MoveToSubdir(outerDir, filepath.Base(subDir)); err != nil {
		t.Fatalf("Error during migration of content to subdirectory: %v", err)
	}
	// validate that the files were moved to the subdirectory
	infos, err := os.ReadDir(subDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 4 {
		t.Fatalf("Should be four files in the subdir after the migration: actual length: %d", len(infos))
	}
	var results []string
	for _, info := range infos {
		results = append(results, info.Name())
	}
	sort.Strings(results)
	if !reflect.DeepEqual(filesList, results) {
		t.Fatalf("Results after migration do not equal list of files: expected: %v, got: %v", filesList, results)
	}
}

// Test a non-existing directory
func TestSizeNonExistingDirectory(t *testing.T) {
	if _, err := Size(context.Background(), "/thisdirectoryshouldnotexist/TestSizeNonExistingDirectory"); err == nil {
		t.Fatalf("error is expected")
	}
}

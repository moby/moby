package directory

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

// Size of an empty directory should be 0
func (s *DockerSuite) TestSizeEmpty(c *check.C) {
	var dir string
	var err error
	if dir, err = ioutil.TempDir(os.TempDir(), "testSizeEmptyDirectory"); err != nil {
		c.Fatalf("failed to create directory: %s", err)
	}

	var size int64
	if size, _ = Size(dir); size != 0 {
		c.Fatalf("empty directory has size: %d", size)
	}
}

// Size of a directory with one empty file should be 0
func (s *DockerSuite) TestSizeEmptyFile(c *check.C) {
	var dir string
	var err error
	if dir, err = ioutil.TempDir(os.TempDir(), "testSizeEmptyFile"); err != nil {
		c.Fatalf("failed to create directory: %s", err)
	}

	var file *os.File
	if file, err = ioutil.TempFile(dir, "file"); err != nil {
		c.Fatalf("failed to create file: %s", err)
	}

	var size int64
	if size, _ = Size(file.Name()); size != 0 {
		c.Fatalf("directory with one file has size: %d", size)
	}
}

// Size of a directory with one 5-byte file should be 5
func (s *DockerSuite) TestSizeNonemptyFile(c *check.C) {
	var dir string
	var err error
	if dir, err = ioutil.TempDir(os.TempDir(), "testSizeNonemptyFile"); err != nil {
		c.Fatalf("failed to create directory: %s", err)
	}

	var file *os.File
	if file, err = ioutil.TempFile(dir, "file"); err != nil {
		c.Fatalf("failed to create file: %s", err)
	}

	d := []byte{97, 98, 99, 100, 101}
	file.Write(d)

	var size int64
	if size, _ = Size(file.Name()); size != 5 {
		c.Fatalf("directory with one 5-byte file has size: %d", size)
	}
}

// Size of a directory with one empty directory should be 0
func (s *DockerSuite) TestSizeNestedDirectoryEmpty(c *check.C) {
	var dir string
	var err error
	if dir, err = ioutil.TempDir(os.TempDir(), "testSizeNestedDirectoryEmpty"); err != nil {
		c.Fatalf("failed to create directory: %s", err)
	}
	if dir, err = ioutil.TempDir(dir, "nested"); err != nil {
		c.Fatalf("failed to create nested directory: %s", err)
	}

	var size int64
	if size, _ = Size(dir); size != 0 {
		c.Fatalf("directory with one empty directory has size: %d", size)
	}
}

// Test directory with 1 file and 1 empty directory
func (s *DockerSuite) TestSizeFileAndNestedDirectoryEmpty(c *check.C) {
	var dir string
	var err error
	if dir, err = ioutil.TempDir(os.TempDir(), "testSizeFileAndNestedDirectoryEmpty"); err != nil {
		c.Fatalf("failed to create directory: %s", err)
	}
	if dir, err = ioutil.TempDir(dir, "nested"); err != nil {
		c.Fatalf("failed to create nested directory: %s", err)
	}

	var file *os.File
	if file, err = ioutil.TempFile(dir, "file"); err != nil {
		c.Fatalf("failed to create file: %s", err)
	}

	d := []byte{100, 111, 99, 107, 101, 114}
	file.Write(d)

	var size int64
	if size, _ = Size(dir); size != 6 {
		c.Fatalf("directory with 6-byte file and empty directory has size: %d", size)
	}
}

// Test directory with 1 file and 1 non-empty directory
func (s *DockerSuite) TestSizeFileAndNestedDirectoryNonempty(c *check.C) {
	var dir, dirNested string
	var err error
	if dir, err = ioutil.TempDir(os.TempDir(), "TestSizeFileAndNestedDirectoryNonempty"); err != nil {
		c.Fatalf("failed to create directory: %s", err)
	}
	if dirNested, err = ioutil.TempDir(dir, "nested"); err != nil {
		c.Fatalf("failed to create nested directory: %s", err)
	}

	var file *os.File
	if file, err = ioutil.TempFile(dir, "file"); err != nil {
		c.Fatalf("failed to create file: %s", err)
	}

	data := []byte{100, 111, 99, 107, 101, 114}
	file.Write(data)

	var nestedFile *os.File
	if nestedFile, err = ioutil.TempFile(dirNested, "file"); err != nil {
		c.Fatalf("failed to create file in nested directory: %s", err)
	}

	nestedData := []byte{100, 111, 99, 107, 101, 114}
	nestedFile.Write(nestedData)

	var size int64
	if size, _ = Size(dir); size != 12 {
		c.Fatalf("directory with 6-byte file and nested directory with 6-byte file has size: %d", size)
	}
}

// Test migration of directory to a subdir underneath itself
func (s *DockerSuite) TestMoveToSubdir(c *check.C) {
	var outerDir, subDir string
	var err error

	if outerDir, err = ioutil.TempDir(os.TempDir(), "TestMoveToSubdir"); err != nil {
		c.Fatalf("failed to create directory: %v", err)
	}

	if subDir, err = ioutil.TempDir(outerDir, "testSub"); err != nil {
		c.Fatalf("failed to create subdirectory: %v", err)
	}

	// write 4 temp files in the outer dir to get moved
	filesList := []string{"a", "b", "c", "d"}
	for _, fName := range filesList {
		if file, err := os.Create(filepath.Join(outerDir, fName)); err != nil {
			c.Fatalf("couldn't create temp file %q: %v", fName, err)
		} else {
			file.WriteString(fName)
			file.Close()
		}
	}

	if err = MoveToSubdir(outerDir, filepath.Base(subDir)); err != nil {
		c.Fatalf("Error during migration of content to subdirectory: %v", err)
	}
	// validate that the files were moved to the subdirectory
	infos, err := ioutil.ReadDir(subDir)
	if err != nil {
		c.Fatal(err)
	}
	if len(infos) != 4 {
		c.Fatalf("Should be four files in the subdir after the migration: actual length: %d", len(infos))
	}
	var results []string
	for _, info := range infos {
		results = append(results, info.Name())
	}
	sort.Sort(sort.StringSlice(results))
	if !reflect.DeepEqual(filesList, results) {
		c.Fatalf("Results after migration do not equal list of files: expected: %v, got: %v", filesList, results)
	}
}

// Test a non-existing directory
func (s *DockerSuite) TestSizeNonExistingDirectory(c *check.C) {
	if _, err := Size("/thisdirectoryshouldnotexist/TestSizeNonExistingDirectory"); err == nil {
		c.Fatalf("error is expected")
	}
}

package builder

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/reexec"
)

const (
	filename = "test"
	contents = "contents test"
)

func init() {
	reexec.Init()
}

func TestCloseRootDirectory(t *testing.T) {
	contextDir, err := ioutil.TempDir("", "builder-tarsum-test")

	if err != nil {
		t.Fatalf("Error with creating temporary directory: %s", err)
	}

	tarsum := &tarSumContext{root: contextDir}

	err = tarsum.Close()

	if err != nil {
		t.Fatalf("Error while executing Close: %s", err)
	}

	_, err = os.Stat(contextDir)

	if !os.IsNotExist(err) {
		t.Fatalf("Directory should not exist at this point")
		defer os.RemoveAll(contextDir)
	}
}

func TestOpenFile(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-tarsum-test")
	defer cleanup()

	createTestTempFile(t, contextDir, filename, contents, 0777)

	tarSum := &tarSumContext{root: contextDir}

	file, err := tarSum.Open(filename)

	if err != nil {
		t.Fatalf("Error when executing Open: %s", err)
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)
	buff := bytes.NewBufferString("")

	for scanner.Scan() {
		buff.WriteString(scanner.Text())
	}

	if contents != buff.String() {
		t.Fatalf("Contents are not equal. Expected: %s, got: %s", contents, buff.String())
	}

}

func TestOpenNotExisting(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-tarsum-test")
	defer cleanup()

	tarSum := &tarSumContext{root: contextDir}

	file, err := tarSum.Open("not-existing")

	if file != nil {
		t.Fatal("Opened file should be nil")
	}

	if !os.IsNotExist(err) {
		t.Fatalf("Error when executing Open: %s", err)
	}
}

func TestStatFile(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-tarsum-test")
	defer cleanup()

	testFilename := createTestTempFile(t, contextDir, filename, contents, 0777)

	tarSum := &tarSumContext{root: contextDir}

	relPath, fileInfo, err := tarSum.Stat(filename)

	if err != nil {
		t.Fatalf("Error when executing Stat: %s", err)
	}

	if relPath != filename {
		t.Fatalf("Relative path should be equal to %s, got %s", filename, relPath)
	}

	if fileInfo.Path() != testFilename {
		t.Fatalf("Full path should be equal to %s, got %s", testFilename, fileInfo.Path())
	}
}

func TestStatSubdir(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-tarsum-test")
	defer cleanup()

	contextSubdir := createTestTempSubdir(t, contextDir, "builder-tarsum-test-subdir")

	testFilename := createTestTempFile(t, contextSubdir, filename, contents, 0777)

	tarSum := &tarSumContext{root: contextDir}

	relativePath, err := filepath.Rel(contextDir, testFilename)

	if err != nil {
		t.Fatalf("Error when getting relative path: %s", err)
	}

	relPath, fileInfo, err := tarSum.Stat(relativePath)

	if err != nil {
		t.Fatalf("Error when executing Stat: %s", err)
	}

	if relPath != relativePath {
		t.Fatalf("Relative path should be equal to %s, got %s", relativePath, relPath)
	}

	if fileInfo.Path() != testFilename {
		t.Fatalf("Full path should be equal to %s, got %s", testFilename, fileInfo.Path())
	}
}

func TestStatNotExisting(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-tarsum-test")
	defer cleanup()

	tarSum := &tarSumContext{root: contextDir}

	relPath, fileInfo, err := tarSum.Stat("not-existing")

	if relPath != "" {
		t.Fatal("Relative path should be nil")
	}

	if fileInfo != nil {
		t.Fatalf("File info should be nil")
	}

	if !os.IsNotExist(err) {
		t.Fatalf("This file should not exist: %s", err)
	}
}

func TestRemoveDirectory(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-tarsum-test")
	defer cleanup()

	contextSubdir := createTestTempSubdir(t, contextDir, "builder-tarsum-test-subdir")

	relativePath, err := filepath.Rel(contextDir, contextSubdir)

	if err != nil {
		t.Fatalf("Error when getting relative path: %s", err)
	}

	tarSum := &tarSumContext{root: contextDir}

	err = tarSum.Remove(relativePath)

	if err != nil {
		t.Fatalf("Error when executing Remove: %s", err)
	}

	_, err = os.Stat(contextSubdir)

	if !os.IsNotExist(err) {
		t.Fatalf("Directory should not exist at this point")
	}
}

func TestMakeSumTarContext(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-tarsum-test")
	defer cleanup()

	createTestTempFile(t, contextDir, filename, contents, 0777)

	tarStream, err := archive.Tar(contextDir, archive.Uncompressed)

	if err != nil {
		t.Fatalf("error: %s", err)
	}

	defer tarStream.Close()

	tarSum, err := MakeTarSumContext(tarStream)

	if err != nil {
		t.Fatalf("Error when executing MakeSumContext: %s", err)
	}

	if tarSum == nil {
		t.Fatalf("Tar sum context should not be nil")
	}
}

func TestWalkWithoutError(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-tarsum-test")
	defer cleanup()

	contextSubdir := createTestTempSubdir(t, contextDir, "builder-tarsum-test-subdir")

	createTestTempFile(t, contextSubdir, filename, contents, 0777)

	tarSum := &tarSumContext{root: contextDir}

	walkFun := func(path string, fi FileInfo, err error) error {
		return nil
	}

	err := tarSum.Walk(contextSubdir, walkFun)

	if err != nil {
		t.Fatalf("Error when executing Walk: %s", err)
	}
}

type WalkError struct {
}

func (we WalkError) Error() string {
	return "Error when executing Walk"
}

func TestWalkWithError(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-tarsum-test")
	defer cleanup()

	contextSubdir := createTestTempSubdir(t, contextDir, "builder-tarsum-test-subdir")

	tarSum := &tarSumContext{root: contextDir}

	walkFun := func(path string, fi FileInfo, err error) error {
		return WalkError{}
	}

	err := tarSum.Walk(contextSubdir, walkFun)

	if err == nil {
		t.Fatalf("Error should not be nil")
	}
}

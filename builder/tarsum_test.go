package builder

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/reexec"
	"github.com/go-check/check"
)

const (
	filename = "test"
	contents = "contents test"
)

func init() {
	reexec.Init()
}

func (s *DockerSuite) TestCloseRootDirectory(c *check.C) {
	contextDir, err := ioutil.TempDir("", "builder-tarsum-test")

	if err != nil {
		c.Fatalf("Error with creating temporary directory: %s", err)
	}

	tarsum := &tarSumContext{root: contextDir}

	err = tarsum.Close()

	if err != nil {
		c.Fatalf("Error while executing Close: %s", err)
	}

	_, err = os.Stat(contextDir)

	if !os.IsNotExist(err) {
		c.Fatalf("Directory should not exist at this point")
		defer os.RemoveAll(contextDir)
	}
}

func (s *DockerSuite) TestOpenFile(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-tarsum-test")
	defer cleanup()

	createTestTempFile(c, contextDir, filename, contents, 0777)

	tarSum := &tarSumContext{root: contextDir}

	file, err := tarSum.Open(filename)

	if err != nil {
		c.Fatalf("Error when executing Open: %s", err)
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)
	buff := bytes.NewBufferString("")

	for scanner.Scan() {
		buff.WriteString(scanner.Text())
	}

	if contents != buff.String() {
		c.Fatalf("Contents are not equal. Expected: %s, got: %s", contents, buff.String())
	}

}

func (s *DockerSuite) TestOpenNotExisting(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-tarsum-test")
	defer cleanup()

	tarSum := &tarSumContext{root: contextDir}

	file, err := tarSum.Open("not-existing")

	if file != nil {
		c.Fatal("Opened file should be nil")
	}

	if !os.IsNotExist(err) {
		c.Fatalf("Error when executing Open: %s", err)
	}
}

func (s *DockerSuite) TestStatFile(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-tarsum-test")
	defer cleanup()

	testFilename := createTestTempFile(c, contextDir, filename, contents, 0777)

	tarSum := &tarSumContext{root: contextDir}

	relPath, fileInfo, err := tarSum.Stat(filename)

	if err != nil {
		c.Fatalf("Error when executing Stat: %s", err)
	}

	if relPath != filename {
		c.Fatalf("Relative path should be equal to %s, got %s", filename, relPath)
	}

	if fileInfo.Path() != testFilename {
		c.Fatalf("Full path should be equal to %s, got %s", testFilename, fileInfo.Path())
	}
}

func (s *DockerSuite) TestStatSubdir(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-tarsum-test")
	defer cleanup()

	contextSubdir := createTestTempSubdir(c, contextDir, "builder-tarsum-test-subdir")

	testFilename := createTestTempFile(c, contextSubdir, filename, contents, 0777)

	tarSum := &tarSumContext{root: contextDir}

	relativePath, err := filepath.Rel(contextDir, testFilename)

	if err != nil {
		c.Fatalf("Error when getting relative path: %s", err)
	}

	relPath, fileInfo, err := tarSum.Stat(relativePath)

	if err != nil {
		c.Fatalf("Error when executing Stat: %s", err)
	}

	if relPath != relativePath {
		c.Fatalf("Relative path should be equal to %s, got %s", relativePath, relPath)
	}

	if fileInfo.Path() != testFilename {
		c.Fatalf("Full path should be equal to %s, got %s", testFilename, fileInfo.Path())
	}
}

func (s *DockerSuite) TestStatNotExisting(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-tarsum-test")
	defer cleanup()

	tarSum := &tarSumContext{root: contextDir}

	relPath, fileInfo, err := tarSum.Stat("not-existing")

	if relPath != "" {
		c.Fatal("Relative path should be nil")
	}

	if fileInfo != nil {
		c.Fatalf("File info should be nil")
	}

	if !os.IsNotExist(err) {
		c.Fatalf("This file should not exist: %s", err)
	}
}

func (s *DockerSuite) TestRemoveDirectory(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-tarsum-test")
	defer cleanup()

	contextSubdir := createTestTempSubdir(c, contextDir, "builder-tarsum-test-subdir")

	relativePath, err := filepath.Rel(contextDir, contextSubdir)

	if err != nil {
		c.Fatalf("Error when getting relative path: %s", err)
	}

	tarSum := &tarSumContext{root: contextDir}

	err = tarSum.Remove(relativePath)

	if err != nil {
		c.Fatalf("Error when executing Remove: %s", err)
	}

	_, err = os.Stat(contextSubdir)

	if !os.IsNotExist(err) {
		c.Fatalf("Directory should not exist at this point")
	}
}

func (s *DockerSuite) TestMakeSumTarContext(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-tarsum-test")
	defer cleanup()

	createTestTempFile(c, contextDir, filename, contents, 0777)

	tarStream, err := archive.Tar(contextDir, archive.Uncompressed)

	if err != nil {
		c.Fatalf("error: %s", err)
	}

	defer tarStream.Close()

	tarSum, err := MakeTarSumContext(tarStream)

	if err != nil {
		c.Fatalf("Error when executing MakeSumContext: %s", err)
	}

	if tarSum == nil {
		c.Fatalf("Tar sum context should not be nil")
	}
}

func (s *DockerSuite) TestWalkWithoutError(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-tarsum-test")
	defer cleanup()

	contextSubdir := createTestTempSubdir(c, contextDir, "builder-tarsum-test-subdir")

	createTestTempFile(c, contextSubdir, filename, contents, 0777)

	tarSum := &tarSumContext{root: contextDir}

	walkFun := func(path string, fi FileInfo, err error) error {
		return nil
	}

	err := tarSum.Walk(contextSubdir, walkFun)

	if err != nil {
		c.Fatalf("Error when executing Walk: %s", err)
	}
}

type WalkError struct {
}

func (we WalkError) Error() string {
	return "Error when executing Walk"
}

func (s *DockerSuite) TestWalkWithError(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-tarsum-test")
	defer cleanup()

	contextSubdir := createTestTempSubdir(c, contextDir, "builder-tarsum-test-subdir")

	tarSum := &tarSumContext{root: contextDir}

	walkFun := func(path string, fi FileInfo, err error) error {
		return WalkError{}
	}

	err := tarSum.Walk(contextSubdir, walkFun)

	if err == nil {
		c.Fatalf("Error should not be nil")
	}
}

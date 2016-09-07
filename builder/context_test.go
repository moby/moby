package builder

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/archive"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

var prepareEmpty = func(c *check.C) (string, func()) {
	return "", func() {}
}

var prepareNoFiles = func(c *check.C) (string, func()) {
	return createTestTempDir(c, "", "builder-context-test")
}

var prepareOneFile = func(c *check.C) (string, func()) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-context-test")
	createTestTempFile(c, contextDir, DefaultDockerfileName, dockerfileContents, 0777)
	return contextDir, cleanup
}

func testValidateContextDirectory(c *check.C, prepare func(c *check.C) (string, func()), excludes []string) {
	contextDir, cleanup := prepare(c)
	defer cleanup()

	err := ValidateContextDirectory(contextDir, excludes)

	if err != nil {
		c.Fatalf("Error should be nil, got: %s", err)
	}
}

func (s *DockerSuite) TestGetContextFromLocalDirNoDockerfile(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-context-test")
	defer cleanup()

	absContextDir, relDockerfile, err := GetContextFromLocalDir(contextDir, "")

	if err == nil {
		c.Fatalf("Error should not be nil")
	}

	if absContextDir != "" {
		c.Fatalf("Absolute directory path should be empty, got: %s", absContextDir)
	}

	if relDockerfile != "" {
		c.Fatalf("Relative path to Dockerfile should be empty, got: %s", relDockerfile)
	}
}

func (s *DockerSuite) TestGetContextFromLocalDirNotExistingDir(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-context-test")
	defer cleanup()

	fakePath := filepath.Join(contextDir, "fake")

	absContextDir, relDockerfile, err := GetContextFromLocalDir(fakePath, "")

	if err == nil {
		c.Fatalf("Error should not be nil")
	}

	if absContextDir != "" {
		c.Fatalf("Absolute directory path should be empty, got: %s", absContextDir)
	}

	if relDockerfile != "" {
		c.Fatalf("Relative path to Dockerfile should be empty, got: %s", relDockerfile)
	}
}

func (s *DockerSuite) TestGetContextFromLocalDirNotExistingDockerfile(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-context-test")
	defer cleanup()

	fakePath := filepath.Join(contextDir, "fake")

	absContextDir, relDockerfile, err := GetContextFromLocalDir(contextDir, fakePath)

	if err == nil {
		c.Fatalf("Error should not be nil")
	}

	if absContextDir != "" {
		c.Fatalf("Absolute directory path should be empty, got: %s", absContextDir)
	}

	if relDockerfile != "" {
		c.Fatalf("Relative path to Dockerfile should be empty, got: %s", relDockerfile)
	}
}

func (s *DockerSuite) TestGetContextFromLocalDirWithNoDirectory(c *check.C) {
	contextDir, dirCleanup := createTestTempDir(c, "", "builder-context-test")
	defer dirCleanup()

	createTestTempFile(c, contextDir, DefaultDockerfileName, dockerfileContents, 0777)

	chdirCleanup := chdir(c, contextDir)
	defer chdirCleanup()

	absContextDir, relDockerfile, err := GetContextFromLocalDir(contextDir, "")

	if err != nil {
		c.Fatalf("Error when getting context from local dir: %s", err)
	}

	if absContextDir != contextDir {
		c.Fatalf("Absolute directory path should be equal to %s, got: %s", contextDir, absContextDir)
	}

	if relDockerfile != DefaultDockerfileName {
		c.Fatalf("Relative path to dockerfile should be equal to %s, got: %s", DefaultDockerfileName, relDockerfile)
	}
}

func (s *DockerSuite) TestGetContextFromLocalDirWithDockerfile(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-context-test")
	defer cleanup()

	createTestTempFile(c, contextDir, DefaultDockerfileName, dockerfileContents, 0777)

	absContextDir, relDockerfile, err := GetContextFromLocalDir(contextDir, "")

	if err != nil {
		c.Fatalf("Error when getting context from local dir: %s", err)
	}

	if absContextDir != contextDir {
		c.Fatalf("Absolute directory path should be equal to %s, got: %s", contextDir, absContextDir)
	}

	if relDockerfile != DefaultDockerfileName {
		c.Fatalf("Relative path to dockerfile should be equal to %s, got: %s", DefaultDockerfileName, relDockerfile)
	}
}

func (s *DockerSuite) TestGetContextFromLocalDirLocalFile(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-context-test")
	defer cleanup()

	createTestTempFile(c, contextDir, DefaultDockerfileName, dockerfileContents, 0777)
	testFilename := createTestTempFile(c, contextDir, "tmpTest", "test", 0777)

	absContextDir, relDockerfile, err := GetContextFromLocalDir(testFilename, "")

	if err == nil {
		c.Fatalf("Error should not be nil")
	}

	if absContextDir != "" {
		c.Fatalf("Absolute directory path should be empty, got: %s", absContextDir)
	}

	if relDockerfile != "" {
		c.Fatalf("Relative path to Dockerfile should be empty, got: %s", relDockerfile)
	}
}

func (s *DockerSuite) TestGetContextFromLocalDirWithCustomDockerfile(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-context-test")
	defer cleanup()

	chdirCleanup := chdir(c, contextDir)
	defer chdirCleanup()

	createTestTempFile(c, contextDir, DefaultDockerfileName, dockerfileContents, 0777)

	absContextDir, relDockerfile, err := GetContextFromLocalDir(contextDir, DefaultDockerfileName)

	if err != nil {
		c.Fatalf("Error when getting context from local dir: %s", err)
	}

	if absContextDir != contextDir {
		c.Fatalf("Absolute directory path should be equal to %s, got: %s", contextDir, absContextDir)
	}

	if relDockerfile != DefaultDockerfileName {
		c.Fatalf("Relative path to dockerfile should be equal to %s, got: %s", DefaultDockerfileName, relDockerfile)
	}

}

func (s *DockerSuite) TestGetContextFromReaderString(c *check.C) {
	tarArchive, relDockerfile, err := GetContextFromReader(ioutil.NopCloser(strings.NewReader(dockerfileContents)), "")

	if err != nil {
		c.Fatalf("Error when executing GetContextFromReader: %s", err)
	}

	tarReader := tar.NewReader(tarArchive)

	_, err = tarReader.Next()

	if err != nil {
		c.Fatalf("Error when reading tar archive: %s", err)
	}

	buff := new(bytes.Buffer)
	buff.ReadFrom(tarReader)
	contents := buff.String()

	_, err = tarReader.Next()

	if err != io.EOF {
		c.Fatalf("Tar stream too long: %s", err)
	}

	if err = tarArchive.Close(); err != nil {
		c.Fatalf("Error when closing tar stream: %s", err)
	}

	if dockerfileContents != contents {
		c.Fatalf("Uncompressed tar archive does not equal: %s, got: %s", dockerfileContents, contents)
	}

	if relDockerfile != DefaultDockerfileName {
		c.Fatalf("Relative path not equals %s, got: %s", DefaultDockerfileName, relDockerfile)
	}
}

func (s *DockerSuite) TestGetContextFromReaderTar(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-context-test")
	defer cleanup()

	createTestTempFile(c, contextDir, DefaultDockerfileName, dockerfileContents, 0777)

	tarStream, err := archive.Tar(contextDir, archive.Uncompressed)

	if err != nil {
		c.Fatalf("Error when creating tar: %s", err)
	}

	tarArchive, relDockerfile, err := GetContextFromReader(tarStream, DefaultDockerfileName)

	if err != nil {
		c.Fatalf("Error when executing GetContextFromReader: %s", err)
	}

	tarReader := tar.NewReader(tarArchive)

	header, err := tarReader.Next()

	if err != nil {
		c.Fatalf("Error when reading tar archive: %s", err)
	}

	if header.Name != DefaultDockerfileName {
		c.Fatalf("Dockerfile name should be: %s, got: %s", DefaultDockerfileName, header.Name)
	}

	buff := new(bytes.Buffer)
	buff.ReadFrom(tarReader)
	contents := buff.String()

	_, err = tarReader.Next()

	if err != io.EOF {
		c.Fatalf("Tar stream too long: %s", err)
	}

	if err = tarArchive.Close(); err != nil {
		c.Fatalf("Error when closing tar stream: %s", err)
	}

	if dockerfileContents != contents {
		c.Fatalf("Uncompressed tar archive does not equal: %s, got: %s", dockerfileContents, contents)
	}

	if relDockerfile != DefaultDockerfileName {
		c.Fatalf("Relative path not equals %s, got: %s", DefaultDockerfileName, relDockerfile)
	}
}

func (s *DockerSuite) TestValidateContextDirectoryEmptyContext(c *check.C) {
	// This isn't a valid test on Windows. See https://play.golang.org/p/RR6z6jxR81.
	// The test will ultimately end up calling filepath.Abs(""). On Windows,
	// golang will error. On Linux, golang will return /. Due to there being
	// drive letters on Windows, this is probably the correct behaviour for
	// Windows.
	if runtime.GOOS == "windows" {
		c.Skip("Invalid test on Windows")
	}
	testValidateContextDirectory(c, prepareEmpty, []string{})
}

func (s *DockerSuite) TestValidateContextDirectoryContextWithNoFiles(c *check.C) {
	testValidateContextDirectory(c, prepareNoFiles, []string{})
}

func (s *DockerSuite) TestValidateContextDirectoryWithOneFile(c *check.C) {
	testValidateContextDirectory(c, prepareOneFile, []string{})
}

func (s *DockerSuite) TestValidateContextDirectoryWithOneFileExcludes(c *check.C) {
	testValidateContextDirectory(c, prepareOneFile, []string{DefaultDockerfileName})
}

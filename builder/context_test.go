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
)

var prepareEmpty = func(t *testing.T) (string, func()) {
	return "", func() {}
}

var prepareNoFiles = func(t *testing.T) (string, func()) {
	return createTestTempDir(t, "", "builder-context-test")
}

var prepareOneFile = func(t *testing.T) (string, func()) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-context-test")
	createTestTempFile(t, contextDir, DefaultDockerfileName, dockerfileContents, 0777)
	return contextDir, cleanup
}

func testValidateContextDirectory(t *testing.T, prepare func(t *testing.T) (string, func()), excludes []string) {
	contextDir, cleanup := prepare(t)
	defer cleanup()

	err := ValidateContextDirectory(contextDir, excludes)

	if err != nil {
		t.Fatalf("Error should be nil, got: %s", err)
	}
}

func TestGetContextFromLocalDirNoDockerfile(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-context-test")
	defer cleanup()

	absContextDir, relDockerfile, err := GetContextFromLocalDir(contextDir, "")

	if err == nil {
		t.Fatalf("Error should not be nil")
	}

	if absContextDir != "" {
		t.Fatalf("Absolute directory path should be empty, got: %s", absContextDir)
	}

	if relDockerfile != "" {
		t.Fatalf("Relative path to Dockerfile should be empty, got: %s", relDockerfile)
	}
}

func TestGetContextFromLocalDirNotExistingDir(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-context-test")
	defer cleanup()

	fakePath := filepath.Join(contextDir, "fake")

	absContextDir, relDockerfile, err := GetContextFromLocalDir(fakePath, "")

	if err == nil {
		t.Fatalf("Error should not be nil")
	}

	if absContextDir != "" {
		t.Fatalf("Absolute directory path should be empty, got: %s", absContextDir)
	}

	if relDockerfile != "" {
		t.Fatalf("Relative path to Dockerfile should be empty, got: %s", relDockerfile)
	}
}

func TestGetContextFromLocalDirNotExistingDockerfile(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-context-test")
	defer cleanup()

	fakePath := filepath.Join(contextDir, "fake")

	absContextDir, relDockerfile, err := GetContextFromLocalDir(contextDir, fakePath)

	if err == nil {
		t.Fatalf("Error should not be nil")
	}

	if absContextDir != "" {
		t.Fatalf("Absolute directory path should be empty, got: %s", absContextDir)
	}

	if relDockerfile != "" {
		t.Fatalf("Relative path to Dockerfile should be empty, got: %s", relDockerfile)
	}
}

func TestGetContextFromLocalDirWithNoDirectory(t *testing.T) {
	contextDir, dirCleanup := createTestTempDir(t, "", "builder-context-test")
	defer dirCleanup()

	createTestTempFile(t, contextDir, DefaultDockerfileName, dockerfileContents, 0777)

	chdirCleanup := chdir(t, contextDir)
	defer chdirCleanup()

	absContextDir, relDockerfile, err := GetContextFromLocalDir(contextDir, "")

	if err != nil {
		t.Fatalf("Error when getting context from local dir: %s", err)
	}

	if absContextDir != contextDir {
		t.Fatalf("Absolute directory path should be equal to %s, got: %s", contextDir, absContextDir)
	}

	if relDockerfile != DefaultDockerfileName {
		t.Fatalf("Relative path to dockerfile should be equal to %s, got: %s", DefaultDockerfileName, relDockerfile)
	}
}

func TestGetContextFromLocalDirWithDockerfile(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-context-test")
	defer cleanup()

	createTestTempFile(t, contextDir, DefaultDockerfileName, dockerfileContents, 0777)

	absContextDir, relDockerfile, err := GetContextFromLocalDir(contextDir, "")

	if err != nil {
		t.Fatalf("Error when getting context from local dir: %s", err)
	}

	if absContextDir != contextDir {
		t.Fatalf("Absolute directory path should be equal to %s, got: %s", contextDir, absContextDir)
	}

	if relDockerfile != DefaultDockerfileName {
		t.Fatalf("Relative path to dockerfile should be equal to %s, got: %s", DefaultDockerfileName, relDockerfile)
	}
}

func TestGetContextFromLocalDirLocalFile(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-context-test")
	defer cleanup()

	createTestTempFile(t, contextDir, DefaultDockerfileName, dockerfileContents, 0777)
	testFilename := createTestTempFile(t, contextDir, "tmpTest", "test", 0777)

	absContextDir, relDockerfile, err := GetContextFromLocalDir(testFilename, "")

	if err == nil {
		t.Fatalf("Error should not be nil")
	}

	if absContextDir != "" {
		t.Fatalf("Absolute directory path should be empty, got: %s", absContextDir)
	}

	if relDockerfile != "" {
		t.Fatalf("Relative path to Dockerfile should be empty, got: %s", relDockerfile)
	}
}

func TestGetContextFromLocalDirWithCustomDockerfile(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-context-test")
	defer cleanup()

	chdirCleanup := chdir(t, contextDir)
	defer chdirCleanup()

	createTestTempFile(t, contextDir, DefaultDockerfileName, dockerfileContents, 0777)

	absContextDir, relDockerfile, err := GetContextFromLocalDir(contextDir, DefaultDockerfileName)

	if err != nil {
		t.Fatalf("Error when getting context from local dir: %s", err)
	}

	if absContextDir != contextDir {
		t.Fatalf("Absolute directory path should be equal to %s, got: %s", contextDir, absContextDir)
	}

	if relDockerfile != DefaultDockerfileName {
		t.Fatalf("Relative path to dockerfile should be equal to %s, got: %s", DefaultDockerfileName, relDockerfile)
	}

}

func TestGetContextFromReaderString(t *testing.T) {
	tarArchive, relDockerfile, err := GetContextFromReader(ioutil.NopCloser(strings.NewReader(dockerfileContents)), "")

	if err != nil {
		t.Fatalf("Error when executing GetContextFromReader: %s", err)
	}

	tarReader := tar.NewReader(tarArchive)

	_, err = tarReader.Next()

	if err != nil {
		t.Fatalf("Error when reading tar archive: %s", err)
	}

	buff := new(bytes.Buffer)
	buff.ReadFrom(tarReader)
	contents := buff.String()

	_, err = tarReader.Next()

	if err != io.EOF {
		t.Fatalf("Tar stream too long: %s", err)
	}

	if err = tarArchive.Close(); err != nil {
		t.Fatalf("Error when closing tar stream: %s", err)
	}

	if dockerfileContents != contents {
		t.Fatalf("Uncompressed tar archive does not equal: %s, got: %s", dockerfileContents, contents)
	}

	if relDockerfile != DefaultDockerfileName {
		t.Fatalf("Relative path not equals %s, got: %s", DefaultDockerfileName, relDockerfile)
	}
}

func TestGetContextFromReaderTar(t *testing.T) {
	contextDir, cleanup := createTestTempDir(t, "", "builder-context-test")
	defer cleanup()

	createTestTempFile(t, contextDir, DefaultDockerfileName, dockerfileContents, 0777)

	tarStream, err := archive.Tar(contextDir, archive.Uncompressed)

	if err != nil {
		t.Fatalf("Error when creating tar: %s", err)
	}

	tarArchive, relDockerfile, err := GetContextFromReader(tarStream, DefaultDockerfileName)

	if err != nil {
		t.Fatalf("Error when executing GetContextFromReader: %s", err)
	}

	tarReader := tar.NewReader(tarArchive)

	header, err := tarReader.Next()

	if err != nil {
		t.Fatalf("Error when reading tar archive: %s", err)
	}

	if header.Name != DefaultDockerfileName {
		t.Fatalf("Dockerfile name should be: %s, got: %s", DefaultDockerfileName, header.Name)
	}

	buff := new(bytes.Buffer)
	buff.ReadFrom(tarReader)
	contents := buff.String()

	_, err = tarReader.Next()

	if err != io.EOF {
		t.Fatalf("Tar stream too long: %s", err)
	}

	if err = tarArchive.Close(); err != nil {
		t.Fatalf("Error when closing tar stream: %s", err)
	}

	if dockerfileContents != contents {
		t.Fatalf("Uncompressed tar archive does not equal: %s, got: %s", dockerfileContents, contents)
	}

	if relDockerfile != DefaultDockerfileName {
		t.Fatalf("Relative path not equals %s, got: %s", DefaultDockerfileName, relDockerfile)
	}
}

func TestValidateContextDirectoryEmptyContext(t *testing.T) {
	// This isn't a valid test on Windows. See https://play.golang.org/p/RR6z6jxR81.
	// The test will ultimately end up calling filepath.Abs(""). On Windows,
	// golang will error. On Linux, golang will return /. Due to there being
	// drive letters on Windows, this is probably the correct behaviour for
	// Windows.
	if runtime.GOOS == "windows" {
		t.Skip("Invalid test on Windows")
	}
	testValidateContextDirectory(t, prepareEmpty, []string{})
}

func TestValidateContextDirectoryContextWithNoFiles(t *testing.T) {
	testValidateContextDirectory(t, prepareNoFiles, []string{})
}

func TestValidateContextDirectoryWithOneFile(t *testing.T) {
	testValidateContextDirectory(t, prepareOneFile, []string{})
}

func TestValidateContextDirectoryWithOneFileExcludes(t *testing.T) {
	testValidateContextDirectory(t, prepareOneFile, []string{DefaultDockerfileName})
}

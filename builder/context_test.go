package builder

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/archive"
)

const (
	dockerfileTestName = "Dockerfile-test"
	dockerfileContent  = "FROM busybox"
)

var prepareEmpty = func(t *testing.T) string {
	return ""
}

var prepareNoFiles = func(t *testing.T) string {
	contextDir, err := ioutil.TempDir("", "builder-context-test")

	if err != nil {
		t.Fatalf("Error when creating temporary directory: %s", err)
	}

	return contextDir
}

var prepareOneFile = func(t *testing.T) string {
	contextDir, err := ioutil.TempDir("", "builder-context-test")

	if err != nil {
		t.Fatalf("Error when creating temporary directory: %s", err)
	}

	dockerfileFilename := filepath.Join(contextDir, dockerfileTestName)
	err = ioutil.WriteFile(dockerfileFilename, []byte(dockerfileContent), 0777)

	if err != nil {
		t.Fatalf("Error with writing to file: %s", err)
	}

	return contextDir
}

func testValidateContextDirectory(t *testing.T, prepare func(t *testing.T) string, excludes []string) {
	contextDir := prepare(t)

	defer os.RemoveAll(contextDir)

	err := ValidateContextDirectory(contextDir, excludes)

	if err != nil {
		t.Fatalf("Error should be nil, got: %s", err)
	}
}

func TestGetContextFromLocalDirNoDockerfile(t *testing.T) {
	contextDir, err := ioutil.TempDir("", "builder-context-test")

	defer os.RemoveAll(contextDir)

	if err != nil {
		t.Fatalf("Error with creating temporary directory: %s", err)
	}

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
	contextDir, err := ioutil.TempDir("", "builder-context-test")

	if err != nil {
		t.Fatalf("Error with creating temporary directory: %s", err)
	}

	defer os.RemoveAll(contextDir)

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
	contextDir, err := ioutil.TempDir("", "builder-context-test")

	if err != nil {
		t.Fatalf("Error with creating temporary directory: %s", err)
	}

	defer os.RemoveAll(contextDir)

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
	contextDir, err := ioutil.TempDir("", "builder-context-test")

	if err != nil {
		t.Fatalf("Error with creating temporary directory: %s", err)
	}

	defer os.RemoveAll(contextDir)

	dockerfileFilename := filepath.Join(contextDir, DefaultDockerfileName)
	err = ioutil.WriteFile(dockerfileFilename, []byte(dockerfileContent), 0777)

	if err != nil {
		t.Fatalf("Error when writing file (%s) contents: %s", dockerfileFilename, err)
	}

	workingDirectory, err := os.Getwd()

	if err != nil {
		t.Fatalf("Error when retrieving working directory: %s", err)
	}

	defer os.Chdir(workingDirectory)

	err = os.Chdir(contextDir)

	if err != nil {
		t.Fatalf("Error when changing directory to %s: %s", contextDir, err)
	}

	absContextDir, relDockerfile, err := GetContextFromLocalDir("", "")

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
	contextDir, err := ioutil.TempDir("", "builder-context-test")

	if err != nil {
		t.Fatalf("Error with creating temporary directory: %s", err)
	}

	defer os.RemoveAll(contextDir)

	dockerfileFilename := filepath.Join(contextDir, DefaultDockerfileName)
	err = ioutil.WriteFile(dockerfileFilename, []byte(dockerfileContent), 0777)

	if err != nil {
		t.Fatalf("Error when writing file (%s) contents: %s", dockerfileFilename, err)
	}

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
	contextDir, err := ioutil.TempDir("", "builder-context-test")

	if err != nil {
		t.Fatalf("Error with creating temporary directory: %s", err)
	}

	defer os.RemoveAll(contextDir)

	dockerfileFilename := filepath.Join(contextDir, DefaultDockerfileName)
	err = ioutil.WriteFile(dockerfileFilename, []byte(dockerfileContent), 0777)

	if err != nil {
		t.Fatalf("Error when writing file (%s) contents: %s", dockerfileFilename, err)
	}

	testFilename := filepath.Join(contextDir, "tmpTest")
	testContent := "test"
	err = ioutil.WriteFile(testFilename, []byte(testContent), 0777)

	if err != nil {
		t.Fatalf("Error when writing file (%s) contents: %s", testFilename, err)
	}

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
	contextDir, err := ioutil.TempDir("", "builder-context-test")

	if err != nil {
		t.Fatalf("Error with creating temporary directory: %s", err)
	}

	defer os.RemoveAll(contextDir)

	workingDirectory, err := os.Getwd()

	if err != nil {
		t.Fatalf("Error when retrieving working directory: %s", err)
	}

	defer os.Chdir(workingDirectory)

	err = os.Chdir(contextDir)

	if err != nil {
		t.Fatalf("Error when changing directory to %s: %s", contextDir, err)
	}

	dockerfileFilename := filepath.Join(contextDir, dockerfileTestName)
	err = ioutil.WriteFile(dockerfileFilename, []byte(dockerfileContent), 0777)

	if err != nil {
		t.Fatalf("Error when writing file (%s) contents: %s", dockerfileFilename, err)
	}

	absContextDir, relDockerfile, err := GetContextFromLocalDir(contextDir, dockerfileTestName)

	if err != nil {
		t.Fatalf("Error when getting context from local dir: %s", err)
	}

	if absContextDir != contextDir {
		t.Fatalf("Absolute directory path should be equal to %s, got: %s", contextDir, absContextDir)
	}

	if relDockerfile != dockerfileTestName {
		t.Fatalf("Relative path to dockerfile should be equal to %s, got: %s", dockerfileTestName, relDockerfile)
	}

}

func TestGetContextFromReaderString(t *testing.T) {
	tarArchive, relDockerfile, err := GetContextFromReader(ioutil.NopCloser(strings.NewReader(dockerfileContent)), "")

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

	if dockerfileContent != contents {
		t.Fatalf("Uncompressed tar archive does not equal: %s, got: %s", dockerfileContent, contents)
	}

	if relDockerfile != DefaultDockerfileName {
		t.Fatalf("Relative path not equals %s, got: %s", DefaultDockerfileName, relDockerfile)
	}
}

func TestGetContextFromReaderTar(t *testing.T) {
	contextDir, err := ioutil.TempDir("", "builder-context-test")

	if err != nil {
		t.Fatalf("Error with creating temporary directory: %s", err)
	}

	defer os.RemoveAll(contextDir)

	dockerfileFilename := filepath.Join(contextDir, dockerfileTestName)
	err = ioutil.WriteFile(dockerfileFilename, []byte(dockerfileContent), 0777)

	if err != nil {
		t.Fatalf("Error when writing file (%s) contents: %s", dockerfileFilename, err)
	}

	tarStream, err := archive.Tar(contextDir, archive.Uncompressed)

	if err != nil {
		t.Fatalf("Error when creating tar: %s", err)
	}

	tarArchive, relDockerfile, err := GetContextFromReader(tarStream, dockerfileTestName)

	if err != nil {
		t.Fatalf("Error when executing GetContextFromReader: %s", err)
	}

	tarReader := tar.NewReader(tarArchive)

	header, err := tarReader.Next()

	if err != nil {
		t.Fatalf("Error when reading tar archive: %s", err)
	}

	if header.Name != dockerfileTestName {
		t.Fatalf("Dockerfile name should be: %s, got: %s", dockerfileTestName, header.Name)
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

	if dockerfileContent != contents {
		t.Fatalf("Uncompressed tar archive does not equal: %s, got: %s", dockerfileContent, contents)
	}

	if relDockerfile != dockerfileTestName {
		t.Fatalf("Relative path not equals %s, got: %s", dockerfileTestName, relDockerfile)
	}
}

func TestValidateContextDirectoryEmptyContext(t *testing.T) {
	testValidateContextDirectory(t, prepareEmpty, []string{})
}

func TestValidateContextDirectoryContextWithNoFiles(t *testing.T) {
	testValidateContextDirectory(t, prepareNoFiles, []string{})
}

func TestValidateContextDirectoryWithOneFile(t *testing.T) {
	testValidateContextDirectory(t, prepareOneFile, []string{})
}

func TestValidateContextDirectoryWithOneFileExcludes(t *testing.T) {
	testValidateContextDirectory(t, prepareOneFile, []string{dockerfileTestName})
}

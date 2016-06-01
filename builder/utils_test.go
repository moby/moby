package builder

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

const (
	dockerfileContents   = "FROM busybox"
	dockerignoreFilename = ".dockerignore"
	testfileContents     = "test"
)

// createTestTempDir creates a temporary directory for testing.
// It returns the created path and a cleanup function which is meant to be used as deferred call.
// When an error occurs, it terminates the test.
func createTestTempDir(t *testing.T, dir, prefix string) (string, func()) {
	path, err := ioutil.TempDir(dir, prefix)

	if err != nil {
		t.Fatalf("Error when creating directory %s with prefix %s: %s", dir, prefix, err)
	}

	return path, func() {
		err = os.RemoveAll(path)

		if err != nil {
			t.Fatalf("Error when removing directory %s: %s", path, err)
		}
	}
}

// createTestTempSubdir creates a temporary directory for testing.
// It returns the created path but doesn't provide a cleanup function,
// so createTestTempSubdir should be used only for creating temporary subdirectories
// whose parent directories are properly cleaned up.
// When an error occurs, it terminates the test.
func createTestTempSubdir(t *testing.T, dir, prefix string) string {
	path, err := ioutil.TempDir(dir, prefix)

	if err != nil {
		t.Fatalf("Error when creating directory %s with prefix %s: %s", dir, prefix, err)
	}

	return path
}

// createTestTempFile creates a temporary file within dir with specific contents and permissions.
// When an error occurs, it terminates the test
func createTestTempFile(t *testing.T, dir, filename, contents string, perm os.FileMode) string {
	filePath := filepath.Join(dir, filename)
	err := ioutil.WriteFile(filePath, []byte(contents), perm)

	if err != nil {
		t.Fatalf("Error when creating %s file: %s", filename, err)
	}

	return filePath
}

// chdir changes current working directory to dir.
// It returns a function which changes working directory back to the previous one.
// This function is meant to be executed as a deferred call.
// When an error occurs, it terminates the test.
func chdir(t *testing.T, dir string) func() {
	workingDirectory, err := os.Getwd()

	if err != nil {
		t.Fatalf("Error when retrieving working directory: %s", err)
	}

	err = os.Chdir(dir)

	if err != nil {
		t.Fatalf("Error when changing directory to %s: %s", dir, err)
	}

	return func() {
		err = os.Chdir(workingDirectory)

		if err != nil {
			t.Fatalf("Error when changing back to working directory (%s): %s", workingDirectory, err)
		}
	}
}

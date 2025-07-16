package dockerfile

import (
	"os"
	"path/filepath"
	"testing"
)

// createTestTempFile creates a temporary file within dir with specific contents and permissions.
// When an error occurs, it terminates the test
func createTestTempFile(t *testing.T, dir, filename, contents string, perm os.FileMode) string {
	filePath := filepath.Join(dir, filename)
	err := os.WriteFile(filePath, []byte(contents), perm)
	if err != nil {
		t.Fatalf("Error when creating %s file: %s", filename, err)
	}

	return filePath
}

// createTestSymlink creates a symlink file within dir which points to oldname
func createTestSymlink(t *testing.T, dir, filename, oldname string) string {
	filePath := filepath.Join(dir, filename)
	if err := os.Symlink(oldname, filePath); err != nil {
		t.Fatalf("Error when creating %s symlink to %s: %s", filename, oldname, err)
	}

	return filePath
}

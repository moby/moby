package testutil // import "github.com/docker/docker/testutil"

import (
	"os"
	"path/filepath"
	"testing"
)

// TempDir returns a temporary directory for use in tests.
// t.TempDir() can't be used as the temporary directory returned by
// that function cannot be accessed by the fake-root user for rootless
// Docker. It creates a nested hierarchy of directories where the
// outermost has permission 0700.
func TempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	parent := filepath.Dir(dir)
	if parent != "" {
		if err := os.Chmod(parent, 0o777); err != nil {
			t.Fatalf("Failed to chmod parent of temp directory %q: %v", parent, err)
		}
	}

	return dir
}

package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"
)

// TestPolicyStateDirPermissions checks that the policy verifier's trust
// directory is created owner-only (0o700) and not exposed to other users.
func TestPolicyStateDirPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission bits are not enforced on Windows")
	}
	root := t.TempDir()
	confDir, err := policyStateDir(root)
	assert.NilError(t, err)
	fi, err := os.Stat(filepath.Join(confDir, "tuf"))
	assert.NilError(t, err)
	assert.Equal(t, fi.Mode().Perm(), os.FileMode(0o700))
}

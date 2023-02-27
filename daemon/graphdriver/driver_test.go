package graphdriver // import "github.com/docker/docker/daemon/graphdriver"

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestIsEmptyDir(t *testing.T) {
	tmp, err := os.MkdirTemp("", "test-is-empty-dir")
	assert.NilError(t, err)
	defer os.RemoveAll(tmp)

	d := filepath.Join(tmp, "empty-dir")
	err = os.Mkdir(d, 0755)
	assert.NilError(t, err)
	empty := isEmptyDir(d)
	assert.Check(t, empty)

	d = filepath.Join(tmp, "dir-with-subdir")
	err = os.MkdirAll(filepath.Join(d, "subdir"), 0755)
	assert.NilError(t, err)
	empty = isEmptyDir(d)
	assert.Check(t, !empty)

	d = filepath.Join(tmp, "dir-with-empty-file")
	err = os.Mkdir(d, 0755)
	assert.NilError(t, err)
	_, err = os.CreateTemp(d, "file")
	assert.NilError(t, err)
	empty = isEmptyDir(d)
	assert.Check(t, !empty)
}

package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestCreateIfNotExists(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		dir := t.TempDir()
		root, err := os.OpenRoot(dir)
		assert.NilError(t, err)
		defer root.Close()

		err = createIfNotExists(root, "tocreate", true)
		assert.NilError(t, err)

		fileinfo, err := os.Stat(filepath.Join(dir, "tocreate"))
		assert.NilError(t, err, "Did not create destination")
		assert.Assert(t, fileinfo.IsDir(), "Should have been a dir, seems it's not")

		err = createIfNotExists(root, "tocreate", true)
		assert.NilError(t, err, "Should not fail if already exists")
	})
	t.Run("file", func(t *testing.T) {
		dir := t.TempDir()
		root, err := os.OpenRoot(dir)
		assert.NilError(t, err)
		defer root.Close()

		err = createIfNotExists(root, "file/to/create", false)
		assert.NilError(t, err)

		fileinfo, err := os.Stat(filepath.Join(dir, "file/to/create"))
		assert.NilError(t, err, "Did not create destination")

		assert.Assert(t, !fileinfo.IsDir(), "Should have been a file, but created a directory")

		err = createIfNotExists(root, "file/to/create", false)
		assert.NilError(t, err, "Should not fail if already exists")
	})
}

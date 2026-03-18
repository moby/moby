package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestCreateIfNotExists(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		toCreate := filepath.Join(t.TempDir(), "tocreate")

		err := createIfNotExists(toCreate, true)
		assert.NilError(t, err)

		fileinfo, err := os.Stat(toCreate)
		assert.NilError(t, err, "Did not create destination")
		assert.Assert(t, fileinfo.IsDir(), "Should have been a dir, seems it's not")

		err = createIfNotExists(toCreate, true)
		assert.NilError(t, err, "Should not fail if already exists")
	})
	t.Run("file", func(t *testing.T) {
		toCreate := filepath.Join(t.TempDir(), "file/to/create")

		err := createIfNotExists(toCreate, false)
		assert.NilError(t, err)

		fileinfo, err := os.Stat(toCreate)
		assert.NilError(t, err, "Did not create destination")

		assert.Assert(t, !fileinfo.IsDir(), "Should have been a file, but created a directory")

		err = createIfNotExists(toCreate, true)
		assert.NilError(t, err, "Should not fail if already exists")
	})
}

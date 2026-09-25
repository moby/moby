package daemon

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
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

	// A mount destination which already exists is left alone whatever its
	// type. Nested bind mounts make the destination of the inner mount
	// resolve to the outer mount's contents, so it can be any kind of file.
	t.Run("existing socket", func(t *testing.T) {
		dir := t.TempDir()
		l, err := net.Listen("unix", filepath.Join(dir, "sock"))
		assert.NilError(t, err)
		defer l.Close()

		root, err := os.OpenRoot(dir)
		assert.NilError(t, err)
		defer root.Close()

		err = createIfNotExists(root, "sock", false)
		assert.NilError(t, err, "Should not fail if already exists as a socket")
	})
	t.Run("existing fifo", func(t *testing.T) {
		dir := t.TempDir()
		assert.NilError(t, unix.Mkfifo(filepath.Join(dir, "fifo"), 0o600))

		root, err := os.OpenRoot(dir)
		assert.NilError(t, err)
		defer root.Close()

		// Opening a FIFO blocks until the other end is opened, so guard
		// against the test hanging for the whole -timeout if it regresses.
		done := make(chan error, 1)
		go func() { done <- createIfNotExists(root, "fifo", false) }()
		select {
		case err := <-done:
			assert.NilError(t, err, "Should not fail if already exists as a FIFO")
		case <-time.After(10 * time.Second):
			t.Fatal("createIfNotExists blocked on an existing FIFO")
		}
	})
	t.Run("existing socket when a directory is expected", func(t *testing.T) {
		dir := t.TempDir()
		l, err := net.Listen("unix", filepath.Join(dir, "sock"))
		assert.NilError(t, err)
		defer l.Close()

		root, err := os.OpenRoot(dir)
		assert.NilError(t, err)
		defer root.Close()

		// Nothing can be bind-mounted onto it -- the kernel rejects a
		// directory source with ENOTDIR -- so it is reported rather than
		// left for mount(2) to fail on.
		err = createIfNotExists(root, "sock", true)
		assert.ErrorIs(t, err, os.ErrExist)
	})
	t.Run("existing directory when a file is expected", func(t *testing.T) {
		dir := t.TempDir()
		assert.NilError(t, os.Mkdir(filepath.Join(dir, "dir"), 0o755))

		root, err := os.OpenRoot(dir)
		assert.NilError(t, err)
		defer root.Close()

		err = createIfNotExists(root, "dir", false)
		assert.NilError(t, err, "Should not fail if already exists as a directory")
	})
}

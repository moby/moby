package loggerutils

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestOpenFileDelete(t *testing.T) {
	tmpDir := t.TempDir()
	f, err := openFile(filepath.Join(tmpDir, "test.txt"), os.O_CREATE|os.O_RDWR, 644)
	assert.NilError(t, err)
	defer f.Close()

	assert.NilError(t, os.RemoveAll(f.Name()))
}

func TestOpenFileRename(t *testing.T) {
	tmpDir := t.TempDir()
	f, err := openFile(filepath.Join(tmpDir, "test.txt"), os.O_CREATE|os.O_RDWR, 0644)
	assert.NilError(t, err)
	defer f.Close()

	assert.NilError(t, os.Rename(f.Name(), f.Name()+"renamed"))
}

func TestUnlinkOpenFile(t *testing.T) {
	tmpDir := t.TempDir()
	name := filepath.Join(tmpDir, "test.txt")
	f, err := openFile(name, os.O_CREATE|os.O_RDWR, 0644)
	assert.NilError(t, err)
	defer func() { assert.NilError(t, f.Close()) }()

	_, err = io.WriteString(f, "first")
	assert.NilError(t, err)

	assert.NilError(t, unlink(name))
	f2, err := openFile(name, os.O_CREATE|os.O_RDWR, 0644)
	assert.NilError(t, err)
	defer func() { assert.NilError(t, f2.Close()) }()

	_, err = io.WriteString(f2, "second")
	assert.NilError(t, err)

	_, err = f.Seek(0, io.SeekStart)
	assert.NilError(t, err)
	fdata, err := io.ReadAll(f)
	assert.NilError(t, err)
	assert.Check(t, "first" == string(fdata))

	_, err = f2.Seek(0, io.SeekStart)
	assert.NilError(t, err)
	f2data, err := io.ReadAll(f2)
	assert.NilError(t, err)
	assert.Check(t, "second" == string(f2data))
}

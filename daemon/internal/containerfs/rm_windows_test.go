package containerfs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
	"gotest.tools/v3/assert"
)

func TestEnsureRemoveAllRetriesSharingViolation(t *testing.T) {
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked")
	assert.NilError(t, os.WriteFile(locked, []byte("locked"), 0o600))

	path, err := windows.UTF16PtrFromString(locked)
	assert.NilError(t, err)

	h, err := windows.CreateFile(
		path,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	assert.NilError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(150 * time.Millisecond)
		_ = windows.CloseHandle(h)
	}()

	assert.NilError(t, EnsureRemoveAll(dir))
	<-done

	_, err = os.Stat(dir)
	assert.Assert(t, os.IsNotExist(err))
}

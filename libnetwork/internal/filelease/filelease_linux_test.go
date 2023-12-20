package filelease

import (
	"os"
	"path"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func appendToFile(fname, content string, ch chan error) {
	f, err := os.OpenFile(fname, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		ch <- err
		return
	}
	defer f.Close()
	_, err = f.Write([]byte(content))
	ch <- err
}

// Check that a file lease can be obtained, and that it delays other writers.
func TestFileLeaseDefaults(t *testing.T) {
	td := t.TempDir()
	fname := path.Join(td, "testfile")

	fl, err := OpenFile(fname, false)
	assert.NilError(t, err)
	assert.Check(t, fl.Leased())

	ch := make(chan error, 1)
	go appendToFile(fname, "second", ch)

	// Sleep while holding the lease to give appendToFile() plenty of time
	// to sneak in its write, if it can.
	time.Sleep(1 * time.Second)
	err = fl.WriteFile([]byte("first"))
	assert.NilError(t, err)
	fl.Close()

	err = <-ch
	assert.NilError(t, err)

	content, _ := os.ReadFile(fname)
	assert.Check(t, cmp.Equal(string(content), "firstsecond"))
}

// Check that, if !mustLease, and a lease cannot be obtained because there is
// already an open file descriptor, the file is still opened.
func TestFileLeaseFail(t *testing.T) {
	td := t.TempDir()
	fname := path.Join(td, "testfile")

	clashFd, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY, 0644)
	assert.NilError(t, err)
	defer clashFd.Close()

	fl, err := OpenFile(fname, false, WithFlags(os.O_WRONLY), WithFileMode(0777))
	assert.NilError(t, err)
	assert.Check(t, !fl.Leased())

	ch := make(chan error, 1)
	go appendToFile(fname, "second", ch)

	time.Sleep(1 * time.Second)
	err = fl.WriteFile([]byte("first"))
	assert.NilError(t, err)
	fl.Close()

	err = <-ch
	assert.NilError(t, err)

	content, _ := os.ReadFile(fname)
	assert.Check(t, cmp.Equal(string(content), "first"))
}

// Check that if a lease cannot be obtained because there is already an open file
// descriptor, the open fails if that is what's required.
func TestFileLeaseMustFail(t *testing.T) {
	td := t.TempDir()
	fname := path.Join(td, "testfile")

	clashFd, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY, 0644)
	assert.NilError(t, err)
	defer clashFd.Close()

	fl, err := OpenFile(fname, true)
	assert.Check(t, fl == nil)
	assert.Check(t, cmp.Error(err, "resource temporarily unavailable"))
}

// Check that if a lease cannot be obtained immediately because there is already
// an open file descriptor, the FileLeaser can wait and retry, if that is what
// is required.
func TestFileLeaseRetry(t *testing.T) {
	td := t.TempDir()
	fname := path.Join(td, "testfile")

	// Create the file and keep an open file descriptor.
	clashFd, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY, 0644)
	assert.NilError(t, err)
	// Close the clashing descriptor after one second.
	go func() {
		time.Sleep(1 * time.Second)
		clashFd.Close()
	}()

	// Immediately try to open and lease the file, a retry will be needed, allow
	// retries for up to 5 seconds.
	start := time.Now()
	fl, err := OpenFile(fname, false, WithRetry(10, 500*time.Millisecond))
	elapsed := time.Since(start)
	assert.NilError(t, err)
	defer fl.Close()
	assert.Check(t, fl.Leased())

	// Check that the lease was acquired soon after the clashing descriptor closed.
	assert.Check(t, elapsed < 2*time.Second)
}

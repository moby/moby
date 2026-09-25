/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

/*
Package atomicfile provides a mechanism (on Unix-like platforms) to present a consistent view of a file to separate
processes even while the file is being written.  This is accomplished by writing a temporary file, syncing to disk, and
renaming over the destination file name.

Partial/inconsistent reads can occur due to:
 1. A process attempting to read the file while it is being written to (both in the case of a new file with a
    short/incomplete write or in the case of an existing, updated file where new bytes may be written at the beginning
    but old bytes may still be present after).
 2. Concurrent goroutines leading to multiple active writers of the same file.

The above mechanism explicitly protects against (1) as all writes are to a file with a temporary name.

There is no explicit protection against multiple, concurrent goroutines attempting to write the same file. However,
atomically writing the file should mean only one writer will "win" and a consistent file will be visible.

Note: atomicfile is partially implemented for Windows. The Windows codepath performs the same operations, however
Windows does not guarantee that a rename operation is atomic; a crash in the middle may leave the destination file
truncated rather than with the expected content.
*/
package atomicfile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// File is an io.ReadWriteCloser that can also be Canceled if a change needs to be abandoned.
type File interface {
	io.ReadWriteCloser
	// Cancel abandons a change to a file. This can be called if a write fails or another error occurs.
	Cancel() error
}

// ErrClosed is returned if Read or Write are called on a closed File.
var ErrClosed = errors.New("file is closed")

// New returns a new atomic file.  On Unix-like platforms, the writer (an io.ReadWriteCloser) is backed by a temporary
// file placed into the same directory as the destination file (using filepath.Dir to split the directory from the
// name).  On a call to Close the temporary file is synced to disk and renamed to its final name, hiding any previous
// file by the same name.
//
// Note: Take care to call Close and handle any errors that are returned.  Errors returned from Close may indicate that
// the file was not written with its final name.
func New(name string, mode os.FileMode) (File, error) {
	return newFile(name, mode)
}

type atomicFile struct {
	name     string
	f        *os.File
	closed   bool
	closedMu sync.RWMutex
}

func newFile(name string, mode os.FileMode) (File, error) {
	dir := filepath.Dir(name)
	f, err := os.CreateTemp(dir, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	if err := f.Chmod(mode); err != nil {
		return nil, fmt.Errorf("failed to change temp file permissions: %w", err)
	}
	return &atomicFile{name: name, f: f}, nil
}

func (a *atomicFile) Close() (err error) {
	a.closedMu.Lock()
	defer a.closedMu.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true

	defer func() {
		if err != nil {
			_ = os.Remove(a.f.Name()) // ignore errors
		}
	}()
	// The order of operations here is:
	// 1. sync
	// 2. close
	// 3. rename
	// While the ordering of 2 and 3 is not important on Unix-like operating systems, Windows cannot rename an open
	// file. By closing first, we allow the rename operation to succeed.
	if err = a.f.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file %q: %w", a.f.Name(), err)
	}
	if err = a.f.Close(); err != nil {
		return fmt.Errorf("failed to close temp file %q: %w", a.f.Name(), err)
	}
	if err = os.Rename(a.f.Name(), a.name); err != nil {
		return fmt.Errorf("failed to rename %q to %q: %w", a.f.Name(), a.name, err)
	}
	return nil
}

func (a *atomicFile) Cancel() error {
	a.closedMu.Lock()
	defer a.closedMu.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true
	_ = a.f.Close() // ignore error
	return os.Remove(a.f.Name())
}

func (a *atomicFile) Read(p []byte) (n int, err error) {
	a.closedMu.RLock()
	defer a.closedMu.RUnlock()
	if a.closed {
		return 0, ErrClosed
	}
	return a.f.Read(p)
}

func (a *atomicFile) Write(p []byte) (n int, err error) {
	a.closedMu.RLock()
	defer a.closedMu.RUnlock()
	if a.closed {
		return 0, ErrClosed
	}
	return a.f.Write(p)
}

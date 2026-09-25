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

package local

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func getATime(fi os.FileInfo) time.Time {
	return fi.ModTime()
}

const (
	// maxFileRetries is the number of attempts for file operations that
	// may fail due to transient sharing violations on Windows.
	maxFileRetries = 5

	// fileRetryDelay is the base delay between retries. Each attempt
	// waits (attempt number) * fileRetryDelay.
	fileRetryDelay = 100 * time.Millisecond
)

// isRetryableError returns true for Windows errors that are known to be
// transient when a file handle has not been fully released by the OS.
func isRetryableError(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) ||
		errors.Is(err, windows.ERROR_LOCK_VIOLATION) ||
		errors.Is(err, windows.ERROR_DIR_NOT_EMPTY)
}

// removePath wraps os.RemoveAll with a retry loop for Windows.
//
// On Windows, a file that has been closed by the application may
// still be held briefly by the OS. Unlike Unix, Windows does not
// allow deleting a file while any handle is open, so os.RemoveAll
// can fail with a sharing violation if it races with a recent Close.
// Retrying after a short delay allows the OS to finish releasing the handle.
func removePath(path string) error {
	var err error
	for i := range maxFileRetries {
		err = os.RemoveAll(path)
		if err == nil {
			return nil
		}
		if !isRetryableError(err) {
			return err
		}
		if i < maxFileRetries-1 {
			time.Sleep(time.Duration(i+1) * fileRetryDelay)
		}
	}
	return err
}

// readFileWithRetry wraps os.ReadFile with a retry loop for Windows.
//
// On Windows, a file that has been closed by the application may
// still be held briefly by the OS. Unlike Unix, Windows does not
// allow reading a file while an exclusive handle is open, so
// os.ReadFile can fail with a sharing violation if it races with
// a recent Close. Retrying after a short delay allows the OS to
// finish releasing the handle.
func readFileWithRetry(path string) ([]byte, error) {
	var err error
	for i := range maxFileRetries {
		var b []byte
		b, err = os.ReadFile(path)
		if err == nil {
			return b, nil
		}
		if os.IsNotExist(err) || !isRetryableError(err) {
			return nil, err
		}
		if i < maxFileRetries-1 {
			time.Sleep(time.Duration(i+1) * fileRetryDelay)
		}
	}
	return nil, err
}

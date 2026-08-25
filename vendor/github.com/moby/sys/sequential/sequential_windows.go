//go:build windows

package sequential

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Create is a copy of [os.Create], modified to use sequential file access.
//
// It uses the Windows sequential scan file flag. Refer to the [Win32 API
// documentation] for details on sequential file access.
//
// [Win32 API documentation]: https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilea#FILE_FLAG_SEQUENTIAL_SCAN
func Create(name string) (*os.File, error) {
	return openFileSequential(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o666)
}

// Open is a copy of [os.Open], modified to use sequential file access.
//
// It uses the Windows sequential scan file flag. Refer to the [Win32 API
// documentation] for details on sequential file access.
//
// [Win32 API documentation]: https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilea#FILE_FLAG_SEQUENTIAL_SCAN
func Open(name string) (*os.File, error) {
	return openFileSequential(name, os.O_RDONLY, 0)
}

// OpenFile is a copy of [os.OpenFile], modified to use sequential file access.
//
// It uses the Windows sequential scan file flag. Refer to the [Win32 API
// documentation] for details on sequential file access.
//
// [Win32 API documentation]: https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilea#FILE_FLAG_SEQUENTIAL_SCAN
func OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return openFileSequential(name, flag, perm)
}

// Helpers for CreateTemp
var (
	rand   uint32
	randmu sync.Mutex
)

func reseed() uint32 {
	return uint32(time.Now().UnixNano() + int64(os.Getpid()))
}

func nextSuffix() string {
	randmu.Lock()
	r := rand
	if r == 0 {
		r = reseed()
	}
	r = r*1664525 + 1013904223 // constants from Numerical Recipes
	rand = r
	randmu.Unlock()
	return strconv.Itoa(int(1e9 + r%1e9))[1:]
}

// CreateTemp is a copy of [os.CreateTemp], modified to use sequential file access.
//
// It uses the Windows sequential scan file flag. Refer to the [Win32 API
// documentation] for details on sequential file access.
//
// [Win32 API documentation]: https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilea#FILE_FLAG_SEQUENTIAL_SCAN
func CreateTemp(dir, prefix string) (f *os.File, err error) {
	if dir == "" {
		dir = os.TempDir()
	}

	nconflict := 0
	for range 10000 {
		name := filepath.Join(dir, prefix+nextSuffix())
		f, err = openFileSequential(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if os.IsExist(err) {
			if nconflict++; nconflict > 10 {
				randmu.Lock()
				rand = reseed()
				randmu.Unlock()
			}
			continue
		}
		break
	}
	return
}

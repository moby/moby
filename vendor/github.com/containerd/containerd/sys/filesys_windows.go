// +build windows

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

package sys

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/Microsoft/hcsshim"
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

const (
	// SddlAdministratorsLocalSystem is local administrators plus NT AUTHORITY\System
	SddlAdministratorsLocalSystem = "D:P(A;OICI;GA;;;BA)(A;OICI;GA;;;SY)"
)

// MkdirAllWithACL is a wrapper for MkdirAll that creates a directory
// ACL'd for Builtin Administrators and Local System.
func MkdirAllWithACL(path string, perm os.FileMode) error {
	return mkdirall(path, true)
}

// MkdirAll implementation that is volume path aware for Windows. It can be used
// as a drop-in replacement for os.MkdirAll()
func MkdirAll(path string, _ os.FileMode) error {
	return mkdirall(path, false)
}

// mkdirall is a custom version of os.MkdirAll modified for use on Windows
// so that it is both volume path aware, and can create a directory with
// a DACL.
func mkdirall(path string, adminAndLocalSystem bool) error {
	if re := regexp.MustCompile(`^\\\\\?\\Volume{[a-z0-9-]+}$`); re.MatchString(path) {
		return nil
	}

	// The rest of this method is largely copied from os.MkdirAll and should be kept
	// as-is to ensure compatibility.

	// Fast path: if we can tell whether path is a directory or file, stop with success or error.
	dir, err := os.Stat(path)
	if err == nil {
		if dir.IsDir() {
			return nil
		}
		return &os.PathError{
			Op:   "mkdir",
			Path: path,
			Err:  syscall.ENOTDIR,
		}
	}

	// Slow path: make sure parent exists and then call Mkdir for path.
	i := len(path)
	for i > 0 && os.IsPathSeparator(path[i-1]) { // Skip trailing path separator.
		i--
	}

	j := i
	for j > 0 && !os.IsPathSeparator(path[j-1]) { // Scan backward over element.
		j--
	}

	if j > 1 {
		// Create parent
		err = mkdirall(path[0:j-1], adminAndLocalSystem)
		if err != nil {
			return err
		}
	}

	// Parent now exists; invoke os.Mkdir or mkdirWithACL and use its result.
	if adminAndLocalSystem {
		err = mkdirWithACL(path)
	} else {
		err = os.Mkdir(path, 0)
	}

	if err != nil {
		// Handle arguments like "foo/." by
		// double-checking that directory doesn't exist.
		dir, err1 := os.Lstat(path)
		if err1 == nil && dir.IsDir() {
			return nil
		}
		return err
	}
	return nil
}

// mkdirWithACL creates a new directory. If there is an error, it will be of
// type *PathError. .
//
// This is a modified and combined version of os.Mkdir and windows.Mkdir
// in golang to cater for creating a directory am ACL permitting full
// access, with inheritance, to any subfolder/file for Built-in Administrators
// and Local System.
func mkdirWithACL(name string) error {
	sa := windows.SecurityAttributes{Length: 0}
	sd, err := windows.SecurityDescriptorFromString(SddlAdministratorsLocalSystem)
	if err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: err}
	}
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1
	sa.SecurityDescriptor = sd

	namep, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: err}
	}

	e := windows.CreateDirectory(namep, &sa)
	if e != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: e}
	}
	return nil
}

// IsAbs is a platform-specific wrapper for filepath.IsAbs. On Windows,
// golang filepath.IsAbs does not consider a path \windows\system32 as absolute
// as it doesn't start with a drive-letter/colon combination. However, in
// docker we need to verify things such as WORKDIR /windows/system32 in
// a Dockerfile (which gets translated to \windows\system32 when being processed
// by the daemon. This SHOULD be treated as absolute from a docker processing
// perspective.
func IsAbs(path string) bool {
	if !filepath.IsAbs(path) {
		if !strings.HasPrefix(path, string(os.PathSeparator)) {
			return false
		}
	}
	return true
}

// The origin of the functions below here are the golang OS and windows packages,
// slightly modified to only cope with files, not directories due to the
// specific use case.
//
// The alteration is to allow a file on Windows to be opened with
// FILE_FLAG_SEQUENTIAL_SCAN (particular for docker load), to avoid eating
// the standby list, particularly when accessing large files such as layer.tar.

// CreateSequential creates the named file with mode 0666 (before umask), truncating
// it if it already exists. If successful, methods on the returned
// File can be used for I/O; the associated file descriptor has mode
// O_RDWR.
// If there is an error, it will be of type *PathError.
func CreateSequential(name string) (*os.File, error) {
	return OpenFileSequential(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0)
}

// OpenSequential opens the named file for reading. If successful, methods on
// the returned file can be used for reading; the associated file
// descriptor has mode O_RDONLY.
// If there is an error, it will be of type *PathError.
func OpenSequential(name string) (*os.File, error) {
	return OpenFileSequential(name, os.O_RDONLY, 0)
}

// OpenFileSequential is the generalized open call; most users will use Open
// or Create instead.
// If there is an error, it will be of type *PathError.
func OpenFileSequential(name string, flag int, _ os.FileMode) (*os.File, error) {
	if name == "" {
		return nil, &os.PathError{Op: "open", Path: name, Err: syscall.ENOENT}
	}
	r, errf := windowsOpenFileSequential(name, flag, 0)
	if errf == nil {
		return r, nil
	}
	return nil, &os.PathError{Op: "open", Path: name, Err: errf}
}

func windowsOpenFileSequential(name string, flag int, _ os.FileMode) (file *os.File, err error) {
	r, e := windowsOpenSequential(name, flag|windows.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	return os.NewFile(uintptr(r), name), nil
}

func makeInheritSa() *windows.SecurityAttributes {
	var sa windows.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1
	return &sa
}

func windowsOpenSequential(path string, mode int, _ uint32) (fd windows.Handle, err error) {
	if len(path) == 0 {
		return windows.InvalidHandle, windows.ERROR_FILE_NOT_FOUND
	}
	pathp, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.InvalidHandle, err
	}
	var access uint32
	switch mode & (windows.O_RDONLY | windows.O_WRONLY | windows.O_RDWR) {
	case windows.O_RDONLY:
		access = windows.GENERIC_READ
	case windows.O_WRONLY:
		access = windows.GENERIC_WRITE
	case windows.O_RDWR:
		access = windows.GENERIC_READ | windows.GENERIC_WRITE
	}
	if mode&windows.O_CREAT != 0 {
		access |= windows.GENERIC_WRITE
	}
	if mode&windows.O_APPEND != 0 {
		access &^= windows.GENERIC_WRITE
		access |= windows.FILE_APPEND_DATA
	}
	sharemode := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE)
	var sa *windows.SecurityAttributes
	if mode&windows.O_CLOEXEC == 0 {
		sa = makeInheritSa()
	}
	var createmode uint32
	switch {
	case mode&(windows.O_CREAT|windows.O_EXCL) == (windows.O_CREAT | windows.O_EXCL):
		createmode = windows.CREATE_NEW
	case mode&(windows.O_CREAT|windows.O_TRUNC) == (windows.O_CREAT | windows.O_TRUNC):
		createmode = windows.CREATE_ALWAYS
	case mode&windows.O_CREAT == windows.O_CREAT:
		createmode = windows.OPEN_ALWAYS
	case mode&windows.O_TRUNC == windows.O_TRUNC:
		createmode = windows.TRUNCATE_EXISTING
	default:
		createmode = windows.OPEN_EXISTING
	}
	// Use FILE_FLAG_SEQUENTIAL_SCAN rather than FILE_ATTRIBUTE_NORMAL as implemented in golang.
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa363858(v=vs.85).aspx
	const fileFlagSequentialScan = 0x08000000 // FILE_FLAG_SEQUENTIAL_SCAN
	h, e := windows.CreateFile(pathp, access, sharemode, sa, createmode, fileFlagSequentialScan, 0)
	return h, e
}

// ForceRemoveAll is the same as os.RemoveAll, but is aware of io.containerd.snapshotter.v1.windows
// and uses hcsshim to unmount and delete container layers contained therein, in the correct order,
// when passed a containerd root data directory (i.e. the `--root` directory for containerd).
func ForceRemoveAll(path string) error {
	// snapshots/windows/windows.go init()
	const snapshotPlugin = "io.containerd.snapshotter.v1" + "." + "windows"
	// snapshots/windows/windows.go NewSnapshotter()
	snapshotDir := filepath.Join(path, snapshotPlugin, "snapshots")
	if stat, err := os.Stat(snapshotDir); err == nil && stat.IsDir() {
		if err := cleanupWCOWLayers(snapshotDir); err != nil {
			return errors.Wrapf(err, "failed to cleanup WCOW layers in %s", snapshotDir)
		}
	}

	return os.RemoveAll(path)
}

func cleanupWCOWLayers(root string) error {
	// See snapshots/windows/windows.go getSnapshotDir()
	var layerNums []int
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if path != root && info.IsDir() {
			if layerNum, err := strconv.Atoi(filepath.Base(path)); err == nil {
				layerNums = append(layerNums, layerNum)
			} else {
				return err
			}
			return filepath.SkipDir
		}

		return nil
	}); err != nil {
		return err
	}

	sort.Sort(sort.Reverse(sort.IntSlice(layerNums)))

	for _, layerNum := range layerNums {
		if err := cleanupWCOWLayer(filepath.Join(root, strconv.Itoa(layerNum))); err != nil {
			return err
		}
	}

	return nil
}

func cleanupWCOWLayer(layerPath string) error {
	info := hcsshim.DriverInfo{
		HomeDir: filepath.Dir(layerPath),
	}

	// ERROR_DEV_NOT_EXIST is returned if the layer is not currently prepared.
	if err := hcsshim.UnprepareLayer(info, filepath.Base(layerPath)); err != nil {
		if hcserror, ok := err.(*hcsshim.HcsError); !ok || hcserror.Err != windows.ERROR_DEV_NOT_EXIST {
			return errors.Wrapf(err, "failed to unprepare %s", layerPath)
		}
	}

	if err := hcsshim.DeactivateLayer(info, filepath.Base(layerPath)); err != nil {
		return errors.Wrapf(err, "failed to deactivate %s", layerPath)
	}

	if err := hcsshim.DestroyLayer(info, filepath.Base(layerPath)); err != nil {
		return errors.Wrapf(err, "failed to destroy %s", layerPath)
	}

	return nil
}

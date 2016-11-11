// +build windows

package system

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"unsafe"

	winio "github.com/Microsoft/go-winio"
)

// MkdirAllWithACL is a wrapper for MkdirAll that creates a directory
// ACL'd for Builtin Administrators and Local System.
func MkdirAllWithACL(path string, perm os.FileMode) error {
	return mkdirall(path, true)
}

// MkdirAll implementation that is volume path aware for Windows.
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
		err = mkdirall(path[0:j-1], false)
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
// This is a modified and combined version of os.Mkdir and syscall.Mkdir
// in golang to cater for creating a directory am ACL permitting full
// access, with inheritance, to any subfolder/file for Built-in Administrators
// and Local System.
func mkdirWithACL(name string) error {
	sa := syscall.SecurityAttributes{Length: 0}
	sddl := "D:P(A;OICI;GA;;;BA)(A;OICI;GA;;;SY)"
	sd, err := winio.SddlToSecurityDescriptor(sddl)
	if err != nil {
		return &os.PathError{"mkdir", name, err}
	}
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1
	sa.SecurityDescriptor = uintptr(unsafe.Pointer(&sd[0]))

	namep, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return &os.PathError{"mkdir", name, err}
	}

	e := syscall.CreateDirectory(namep, &sa)
	if e != nil {
		return &os.PathError{"mkdir", name, e}
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

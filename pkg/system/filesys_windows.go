package system // import "github.com/docker/docker/pkg/system"

import (
	"os"
	"regexp"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SddlAdministratorsLocalSystem is local administrators plus NT AUTHORITY\System.
const SddlAdministratorsLocalSystem = "D:P(A;OICI;GA;;;BA)(A;OICI;GA;;;SY)"

// volumePath is a regular expression to check if a path is a Windows
// volume path (e.g., "\\?\Volume{4c1b02c1-d990-11dc-99ae-806e6f6e6963}"
// or "\\?\Volume{4c1b02c1-d990-11dc-99ae-806e6f6e6963}\").
var volumePath = regexp.MustCompile(`^\\\\\?\\Volume{[a-z0-9-]+}\\?$`)

// MkdirAllWithACL is a custom version of os.MkdirAll modified for use on Windows
// so that it is both volume path aware, and can create a directory with
// an appropriate SDDL defined ACL.
func MkdirAllWithACL(path string, _ os.FileMode, sddl string) error {
	sa, err := makeSecurityAttributes(sddl)
	if err != nil {
		return &os.PathError{Op: "mkdirall", Path: path, Err: err}
	}
	return mkdirall(path, sa)
}

// MkdirAll is a custom version of os.MkdirAll that is volume path aware for
// Windows. It can be used as a drop-in replacement for os.MkdirAll.
func MkdirAll(path string, _ os.FileMode) error {
	return mkdirall(path, nil)
}

// mkdirall is a custom version of os.MkdirAll modified for use on Windows
// so that it is both volume path aware, and can create a directory with
// a DACL.
func mkdirall(path string, perm *windows.SecurityAttributes) error {
	if volumePath.MatchString(path) {
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
		return &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
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
		// Create parent.
		err = mkdirall(fixRootDirectory(path[:j-1]), perm)
		if err != nil {
			return err
		}
	}

	// Parent now exists; invoke Mkdir and use its result.
	err = mkdirWithACL(path, perm)
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
func mkdirWithACL(name string, sa *windows.SecurityAttributes) error {
	if sa == nil {
		return os.Mkdir(name, 0)
	}

	namep, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: err}
	}

	err = windows.CreateDirectory(namep, sa)
	if err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: err}
	}
	return nil
}

// fixRootDirectory fixes a reference to a drive's root directory to
// have the required trailing slash.
func fixRootDirectory(p string) string {
	if len(p) == len(`\\?\c:`) {
		if os.IsPathSeparator(p[0]) && os.IsPathSeparator(p[1]) && p[2] == '?' && os.IsPathSeparator(p[3]) && p[5] == ':' {
			return p + `\`
		}
	}
	return p
}

func makeSecurityAttributes(sddl string) (*windows.SecurityAttributes, error) {
	var sa windows.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1
	var err error
	sa.SecurityDescriptor, err = windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return nil, err
	}
	return &sa, nil
}

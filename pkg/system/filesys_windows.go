package system // import "github.com/docker/docker/pkg/system"

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SddlAdministratorsLocalSystem is local administrators plus NT AUTHORITY\System.
const SddlAdministratorsLocalSystem = "D:P(A;OICI;GA;;;BA)(A;OICI;GA;;;SY)"

// MkdirAllWithACL is a custom version of os.MkdirAll modified for use on Windows
// so that it is both volume path aware, and can create a directory with
// an appropriate SDDL defined ACL.
func MkdirAllWithACL(path string, _ os.FileMode, sddl string) error {
	sa, err := makeSecurityAttributes(sddl)
	if err != nil {
		return &os.PathError{Op: "mkdirall", Path: path, Err: err}
	}
	return mkdirAllWithACL(path, sa)
}

// mkdirAllWithACL is a custom version of os.MkdirAll with DACL support on Windows.
// It is fully identical to [os.MkdirAll] if no DACL is provided.
//
// Code in this function is based on the implementation in [go1.23.4].
//
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// [go1.23.4]: https://github.com/golang/go/blob/go1.23.4/src/os/path.go#L12-L66
func mkdirAllWithACL(path string, perm *windows.SecurityAttributes) error {
	if perm == nil {
		return os.MkdirAll(path, 0)
	}
	// Fast path: if we can tell whether path is a directory or file, stop with success or error.
	dir, err := os.Stat(path)
	if err == nil {
		if dir.IsDir() {
			return nil
		}
		return &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	}

	// Slow path: make sure parent exists and then call Mkdir for path.

	// Extract the parent folder from path by first removing any trailing
	// path separator and then scanning backward until finding a path
	// separator or reaching the beginning of the string.
	i := len(path) - 1
	for i >= 0 && os.IsPathSeparator(path[i]) {
		i--
	}
	for i >= 0 && !os.IsPathSeparator(path[i]) {
		i--
	}
	if i < 0 {
		i = 0
	}

	// If there is a parent directory, and it is not the volume name,
	// recurse to ensure parent directory exists.
	if parent := path[:i]; len(parent) > len(filepath.VolumeName(path)) {
		err = mkdirAllWithACL(parent, perm)
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

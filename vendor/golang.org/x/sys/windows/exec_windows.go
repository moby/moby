// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Fork, exec, wait, etc.

package windows

import (
	errorspkg "errors"
	"unsafe"
)

// EscapeArg rewrites command line argument s as prescribed
// in http://msdn.microsoft.com/en-us/library/ms880421.
// This function returns "" (2 double quotes) if s is empty.
// Alternatively, these transformations are done:
// - every back slash (\) is doubled, but only if immediately
//   followed by double quote (");
// - every double quote (") is escaped by back slash (\);
// - finally, s is wrapped with double quotes (arg -> "arg"),
//   but only if there is space or tab inside s.
func EscapeArg(s string) string {
	if len(s) == 0 {
		return "\"\""
	}
	n := len(s)
	hasSpace := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"', '\\':
			n++
		case ' ', '\t':
			hasSpace = true
		}
	}
	if hasSpace {
		n += 2
	}
	if n == len(s) {
		return s
	}

	qs := make([]byte, n)
	j := 0
	if hasSpace {
		qs[j] = '"'
		j++
	}
	slashes := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		default:
			slashes = 0
			qs[j] = s[i]
		case '\\':
			slashes++
			qs[j] = s[i]
		case '"':
			for ; slashes > 0; slashes-- {
				qs[j] = '\\'
				j++
			}
			qs[j] = '\\'
			j++
			qs[j] = s[i]
		}
		j++
	}
	if hasSpace {
		for ; slashes > 0; slashes-- {
			qs[j] = '\\'
			j++
		}
		qs[j] = '"'
		j++
	}
	return string(qs[:j])
}

func CloseOnExec(fd Handle) {
	SetHandleInformation(Handle(fd), HANDLE_FLAG_INHERIT, 0)
}

// FullPath retrieves the full path of the specified file.
func FullPath(name string) (path string, err error) {
	p, err := UTF16PtrFromString(name)
	if err != nil {
		return "", err
	}
	n := uint32(100)
	for {
		buf := make([]uint16, n)
		n, err = GetFullPathName(p, uint32(len(buf)), &buf[0], nil)
		if err != nil {
			return "", err
		}
		if n <= uint32(len(buf)) {
			return UTF16ToString(buf[:n]), nil
		}
	}
}

// NewProcThreadAttributeList allocates a new ProcThreadAttributeList, with the requested maximum number of attributes.
func NewProcThreadAttributeList(maxAttrCount uint32) (*ProcThreadAttributeList, error) {
	var size uintptr
	err := initializeProcThreadAttributeList(nil, maxAttrCount, 0, &size)
	if err != ERROR_INSUFFICIENT_BUFFER {
		if err == nil {
			return nil, errorspkg.New("unable to query buffer size from InitializeProcThreadAttributeList")
		}
		return nil, err
	}
	const psize = unsafe.Sizeof(uintptr(0))
	// size is guaranteed to be ≥1 by InitializeProcThreadAttributeList.
	al := (*ProcThreadAttributeList)(unsafe.Pointer(&make([]unsafe.Pointer, (size+psize-1)/psize)[0]))
	err = initializeProcThreadAttributeList(al, maxAttrCount, 0, &size)
	if err != nil {
		return nil, err
	}
	return al, err
}

// Update modifies the ProcThreadAttributeList using UpdateProcThreadAttribute.
func (al *ProcThreadAttributeList) Update(attribute uintptr, flags uint32, value unsafe.Pointer, size uintptr, prevValue unsafe.Pointer, returnedSize *uintptr) error {
	return updateProcThreadAttribute(al, flags, attribute, value, size, prevValue, returnedSize)
}

// Delete frees ProcThreadAttributeList's resources.
func (al *ProcThreadAttributeList) Delete() {
	deleteProcThreadAttributeList(al)
}

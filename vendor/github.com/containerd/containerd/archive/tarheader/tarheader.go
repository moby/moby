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
   Portions from https://github.com/moby/moby/blob/v23.0.1/pkg/archive/archive.go#L419-L464
   Copyright (C) Docker/Moby authors.
   Licensed under the Apache License, Version 2.0
   NOTICE: https://github.com/moby/moby/blob/v23.0.1/NOTICE
*/

package tarheader

import (
	"archive/tar"
	"os"
)

// nosysFileInfo hides the system-dependent info of the wrapped FileInfo to
// prevent tar.FileInfoHeader from introspecting it and potentially calling into
// glibc.
//
// From https://github.com/moby/moby/blob/v23.0.1/pkg/archive/archive.go#L419-L434 .
type nosysFileInfo struct {
	os.FileInfo
}

func (fi nosysFileInfo) Sys() interface{} {
	// A Sys value of type *tar.Header is safe as it is system-independent.
	// The tar.FileInfoHeader function copies the fields into the returned
	// header without performing any OS lookups.
	if sys, ok := fi.FileInfo.Sys().(*tar.Header); ok {
		return sys
	}
	return nil
}

// sysStat, if non-nil, populates hdr from system-dependent fields of fi.
//
// From https://github.com/moby/moby/blob/v23.0.1/pkg/archive/archive.go#L436-L437 .
var sysStat func(fi os.FileInfo, hdr *tar.Header) error

// FileInfoHeaderNoLookups creates a partially-populated tar.Header from fi.
//
// Compared to the archive/tar.FileInfoHeader function, this function is safe to
// call from a chrooted process as it does not populate fields which would
// require operating system lookups. It behaves identically to
// tar.FileInfoHeader when fi is a FileInfo value returned from
// tar.Header.FileInfo().
//
// When fi is a FileInfo for a native file, such as returned from os.Stat() and
// os.Lstat(), the returned Header value differs from one returned from
// tar.FileInfoHeader in the following ways. The Uname and Gname fields are not
// set as OS lookups would be required to populate them. The AccessTime and
// ChangeTime fields are not currently set (not yet implemented) although that
// is subject to change. Callers which require the AccessTime or ChangeTime
// fields to be zeroed should explicitly zero them out in the returned Header
// value to avoid any compatibility issues in the future.
//
// From https://github.com/moby/moby/blob/v23.0.1/pkg/archive/archive.go#L439-L464 .
func FileInfoHeaderNoLookups(fi os.FileInfo, link string) (*tar.Header, error) {
	hdr, err := tar.FileInfoHeader(nosysFileInfo{fi}, link)
	if err != nil {
		return nil, err
	}
	if sysStat != nil {
		return hdr, sysStat(fi, hdr)
	}
	return hdr, nil
}

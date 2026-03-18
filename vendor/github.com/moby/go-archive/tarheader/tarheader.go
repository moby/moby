package tarheader

import (
	"archive/tar"
	"os"
)

// assert that we implement [tar.FileInfoNames].
var _ tar.FileInfoNames = (*nosysFileInfo)(nil)

// nosysFileInfo hides the system-dependent info of the wrapped FileInfo to
// prevent tar.FileInfoHeader from introspecting it and potentially calling into
// glibc.
//
// It implements [tar.FileInfoNames] to further prevent [tar.FileInfoHeader]
// from performing any lookups on go1.23 and up. see https://go.dev/issue/50102
type nosysFileInfo struct {
	os.FileInfo
}

// Uname stubs out looking up username. It implements [tar.FileInfoNames]
// to prevent [tar.FileInfoHeader] from loading libraries to perform
// username lookups.
func (fi nosysFileInfo) Uname() (string, error) {
	return "", nil
}

// Gname stubs out looking up group-name. It implements [tar.FileInfoNames]
// to prevent [tar.FileInfoHeader] from loading libraries to perform
// username lookups.
func (fi nosysFileInfo) Gname() (string, error) {
	return "", nil
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
func FileInfoHeaderNoLookups(fi os.FileInfo, link string) (*tar.Header, error) {
	hdr, err := tar.FileInfoHeader(nosysFileInfo{fi}, link)
	if err != nil {
		return nil, err
	}
	return hdr, sysStat(fi, hdr)
}

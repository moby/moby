//go:build go1.17
// +build go1.17

package plugins

import "io/fs"

var fileInfoToDirEntry = fs.FileInfoToDirEntry

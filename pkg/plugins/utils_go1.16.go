//go:build !go1.17
// +build !go1.17

// This code is taken from https://github.com/golang/go/blob/go1.17/src/io/fs/readdir.go#L49-L77
// and provides the io/fs.FileInfoToDirEntry() utility for go1.16. Go 1.16 and up
// provide a new implementation of ioutil.ReadDir() (in os.ReadDir()) that returns
// an os.DirEntry instead of fs.FileInfo. go1.17 added the io/fs.FileInfoToDirEntry()
// utility to allow existing uses of ReadDir() to get the old type. This utility
// is not available in go1.16, so we copied it to assist the migration to os.ReadDir().

// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plugins

import "os"

// dirInfo is a DirEntry based on a FileInfo.
type dirInfo struct {
	fileInfo os.FileInfo
}

func (di dirInfo) IsDir() bool {
	return di.fileInfo.IsDir()
}

func (di dirInfo) Type() os.FileMode {
	return di.fileInfo.Mode().Type()
}

func (di dirInfo) Info() (os.FileInfo, error) {
	return di.fileInfo, nil
}

func (di dirInfo) Name() string {
	return di.fileInfo.Name()
}

// fileInfoToDirEntry returns a DirEntry that returns information from info.
// If info is nil, fileInfoToDirEntry returns nil.
func fileInfoToDirEntry(info os.FileInfo) os.DirEntry {
	if info == nil {
		return nil
	}
	return dirInfo{fileInfo: info}
}

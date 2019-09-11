// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pathfs

import (
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/internal/utimens"
)

func (fs *loopbackFileSystem) Utimens(path string, a *time.Time, m *time.Time, context *fuse.Context) fuse.Status {
	// MacOS before High Sierra lacks utimensat() and UTIME_OMIT.
	// We emulate using utimes() and extra GetAttr() calls.
	var attr *fuse.Attr
	if a == nil || m == nil {
		var status fuse.Status
		attr, status = fs.GetAttr(path, context)
		if !status.Ok() {
			return status
		}
	}
	tv := utimens.Fill(a, m, attr)
	err := syscall.Utimes(fs.GetPath(path), tv)
	return fuse.ToStatus(err)
}

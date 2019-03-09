// Copyright 2018 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package utimens

import (
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/fuse"
)

// timeToTimeval converts time.Time to syscall.Timeval
func timeToTimeval(t *time.Time) syscall.Timeval {
	// Note: This does not use syscall.NsecToTimespec because
	// that does not work properly for times before 1970,
	// see https://github.com/golang/go/issues/12777
	var tv syscall.Timeval
	tv.Usec = int32(t.Nanosecond() / 1000)
	tv.Sec = t.Unix()
	return tv
}

// Fill converts a and m to a syscall.Timeval slice that can be passed
// to syscall.Utimes. Missing values (if any) are taken from attr
func Fill(a *time.Time, m *time.Time, attr *fuse.Attr) []syscall.Timeval {
	if a == nil {
		a2 := time.Unix(int64(attr.Atime), int64(attr.Atimensec))
		a = &a2
	}
	if m == nil {
		m2 := time.Unix(int64(attr.Mtime), int64(attr.Mtimensec))
		m = &m2
	}
	tv := make([]syscall.Timeval, 2)
	tv[0] = timeToTimeval(a)
	tv[1] = timeToTimeval(m)
	return tv
}

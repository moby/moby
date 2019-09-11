// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nodefs

import (
	"syscall"
	"time"
	"unsafe"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/internal/utimens"
)

func (f *loopbackFile) Allocate(off uint64, sz uint64, mode uint32) fuse.Status {
	// TODO: Handle `mode` parameter.

	// From `man fcntl` on OSX:
	//     The F_PREALLOCATE command operates on the following structure:
	//
	//             typedef struct fstore {
	//                 u_int32_t fst_flags;      /* IN: flags word */
	//                 int       fst_posmode;    /* IN: indicates offset field */
	//                 off_t     fst_offset;     /* IN: start of the region */
	//                 off_t     fst_length;     /* IN: size of the region */
	//                 off_t     fst_bytesalloc; /* OUT: number of bytes allocated */
	//             } fstore_t;
	//
	//     The flags (fst_flags) for the F_PREALLOCATE command are as follows:
	//
	//           F_ALLOCATECONTIG   Allocate contiguous space.
	//
	//           F_ALLOCATEALL      Allocate all requested space or no space at all.
	//
	//     The position modes (fst_posmode) for the F_PREALLOCATE command indicate how to use the offset field.  The modes are as fol-
	//     lows:
	//
	//           F_PEOFPOSMODE   Allocate from the physical end of file.
	//
	//           F_VOLPOSMODE    Allocate from the volume offset.

	k := struct {
		Flags      uint32 // u_int32_t
		Posmode    int64  // int
		Offset     int64  // off_t
		Length     int64  // off_t
		Bytesalloc int64  // off_t
	}{
		0,
		0,
		int64(off),
		int64(sz),
		0,
	}

	// Linux version for reference:
	// err := syscall.Fallocate(int(f.File.Fd()), mode, int64(off), int64(sz))

	f.lock.Lock()
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, f.File.Fd(), uintptr(syscall.F_PREALLOCATE), uintptr(unsafe.Pointer(&k)))
	f.lock.Unlock()
	if errno != 0 {
		return fuse.ToStatus(errno)
	}
	return fuse.OK
}

// timeToTimeval - Convert time.Time to syscall.Timeval
//
// Note: This does not use syscall.NsecToTimespec because
// that does not work properly for times before 1970,
// see https://github.com/golang/go/issues/12777
func timeToTimeval(t *time.Time) syscall.Timeval {
	var tv syscall.Timeval
	tv.Usec = int32(t.Nanosecond() / 1000)
	tv.Sec = t.Unix()
	return tv
}

// MacOS before High Sierra lacks utimensat() and UTIME_OMIT.
// We emulate using utimes() and extra GetAttr() calls.
func (f *loopbackFile) Utimens(a *time.Time, m *time.Time) fuse.Status {
	var attr fuse.Attr
	if a == nil || m == nil {
		var status fuse.Status
		status = f.GetAttr(&attr)
		if !status.Ok() {
			return status
		}
	}
	tv := utimens.Fill(a, m, &attr)
	f.lock.Lock()
	err := syscall.Futimes(int(f.File.Fd()), tv)
	f.lock.Unlock()
	return fuse.ToStatus(err)
}

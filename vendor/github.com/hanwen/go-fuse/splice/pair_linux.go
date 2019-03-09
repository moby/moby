// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package splice

import (
	"fmt"
	"os"
	"syscall"
)

func (p *Pair) LoadFromAt(fd uintptr, sz int, off int64) (int, error) {
	n, err := syscall.Splice(int(fd), &off, p.w, nil, sz, 0)
	return int(n), err
}

func (p *Pair) LoadFrom(fd uintptr, sz int) (int, error) {
	if sz > p.size {
		return 0, fmt.Errorf("LoadFrom: not enough space %d, %d",
			sz, p.size)
	}

	n, err := syscall.Splice(int(fd), nil, p.w, nil, sz, 0)
	if err != nil {
		err = os.NewSyscallError("Splice load from", err)
	}
	return int(n), err
}

func (p *Pair) WriteTo(fd uintptr, n int) (int, error) {
	m, err := syscall.Splice(p.r, nil, int(fd), nil, int(n), 0)
	if err != nil {
		err = os.NewSyscallError("Splice write", err)
	}
	return int(m), err
}

const _SPLICE_F_NONBLOCK = 0x2

func (p *Pair) discard() {
	_, err := syscall.Splice(p.r, nil, int(devNullFD), nil, int(p.size), _SPLICE_F_NONBLOCK)
	if err == syscall.EAGAIN {
		// all good.
	} else if err != nil {
		panic(err)
	}
}

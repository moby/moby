// +build !windows

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

package fifo

import (
	"syscall"
)

// SyscallConn provides raw access to the fifo's underlying filedescrptor.
// See syscall.Conn for guarantees provided by this interface.
func (f *fifo) SyscallConn() (syscall.RawConn, error) {
	// deterministic check for closed
	select {
	case <-f.closed:
		return nil, ErrClosed
	default:
	}

	select {
	case <-f.closed:
		return nil, ErrClosed
	case <-f.opened:
		return f.file.SyscallConn()
	default:
	}

	// Not opened and not closed, this means open is non-blocking AND it's not open yet
	// Use rawConn to deal with non-blocking open.
	rc := &rawConn{f: f, ready: make(chan struct{})}
	go func() {
		select {
		case <-f.closed:
			return
		case <-f.opened:
			rc.raw, rc.err = f.file.SyscallConn()
			close(rc.ready)
		}
	}()

	return rc, nil
}

type rawConn struct {
	f     *fifo
	ready chan struct{}
	raw   syscall.RawConn
	err   error
}

func (r *rawConn) Control(f func(fd uintptr)) error {
	select {
	case <-r.f.closed:
		return ErrCtrlClosed
	case <-r.ready:
	}

	if r.err != nil {
		return r.err
	}

	return r.raw.Control(f)
}

func (r *rawConn) Read(f func(fd uintptr) (done bool)) error {
	if r.f.flag&syscall.O_WRONLY > 0 {
		return ErrRdFrmWRONLY
	}

	select {
	case <-r.f.closed:
		return ErrReadClosed
	case <-r.ready:
	}

	if r.err != nil {
		return r.err
	}

	return r.raw.Read(f)
}

func (r *rawConn) Write(f func(fd uintptr) (done bool)) error {
	if r.f.flag&(syscall.O_WRONLY|syscall.O_RDWR) == 0 {
		return ErrWrToRDONLY
	}

	select {
	case <-r.f.closed:
		return ErrWriteClosed
	case <-r.ready:
	}

	if r.err != nil {
		return r.err
	}

	return r.raw.Write(f)
}

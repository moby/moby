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
	"context"
	"io"
	"os"
	"runtime"
	"sync"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

type fifo struct {
	flag        int
	opened      chan struct{}
	closed      chan struct{}
	closing     chan struct{}
	err         error
	file        *os.File
	closingOnce sync.Once // close has been called
	closedOnce  sync.Once // fifo is closed
	handle      *handle
}

var leakCheckWg *sync.WaitGroup

// OpenFifoDup2 is same as OpenFifo, but additionally creates a copy of the FIFO file descriptor with dup2 syscall.
func OpenFifoDup2(ctx context.Context, fn string, flag int, perm os.FileMode, fd int) (io.ReadWriteCloser, error) {
	f, err := openFifo(ctx, fn, flag, perm)
	if err != nil {
		return nil, errors.Wrap(err, "fifo error")
	}

	if err := unix.Dup2(int(f.file.Fd()), fd); err != nil {
		_ = f.Close()
		return nil, errors.Wrap(err, "dup2 error")
	}

	return f, nil
}

// OpenFifo opens a fifo. Returns io.ReadWriteCloser.
// Context can be used to cancel this function until open(2) has not returned.
// Accepted flags:
// - syscall.O_CREAT - create new fifo if one doesn't exist
// - syscall.O_RDONLY - open fifo only from reader side
// - syscall.O_WRONLY - open fifo only from writer side
// - syscall.O_RDWR - open fifo from both sides, never block on syscall level
// - syscall.O_NONBLOCK - return io.ReadWriteCloser even if other side of the
//     fifo isn't open. read/write will be connected after the actual fifo is
//     open or after fifo is closed.
func OpenFifo(ctx context.Context, fn string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	return openFifo(ctx, fn, flag, perm)
}

func openFifo(ctx context.Context, fn string, flag int, perm os.FileMode) (*fifo, error) {
	if _, err := os.Stat(fn); err != nil {
		if os.IsNotExist(err) && flag&syscall.O_CREAT != 0 {
			if err := syscall.Mkfifo(fn, uint32(perm&os.ModePerm)); err != nil && !os.IsExist(err) {
				return nil, errors.Wrapf(err, "error creating fifo %v", fn)
			}
		} else {
			return nil, err
		}
	}

	block := flag&syscall.O_NONBLOCK == 0 || flag&syscall.O_RDWR != 0

	flag &= ^syscall.O_CREAT
	flag &= ^syscall.O_NONBLOCK

	h, err := getHandle(fn)
	if err != nil {
		return nil, err
	}

	f := &fifo{
		handle:  h,
		flag:    flag,
		opened:  make(chan struct{}),
		closed:  make(chan struct{}),
		closing: make(chan struct{}),
	}

	wg := leakCheckWg
	if wg != nil {
		wg.Add(2)
	}

	go func() {
		if wg != nil {
			defer wg.Done()
		}
		select {
		case <-ctx.Done():
			select {
			case <-f.opened:
			default:
				f.Close()
			}
		case <-f.opened:
		case <-f.closed:
		}
	}()
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		var file *os.File
		fn, err := h.Path()
		if err == nil {
			file, err = os.OpenFile(fn, flag, 0)
		}
		select {
		case <-f.closing:
			if err == nil {
				select {
				case <-ctx.Done():
					err = ctx.Err()
				default:
					err = errors.Errorf("fifo %v was closed before opening", h.Name())
				}
				if file != nil {
					file.Close()
				}
			}
		default:
		}
		if err != nil {
			f.closedOnce.Do(func() {
				f.err = err
				close(f.closed)
			})
			return
		}
		f.file = file
		close(f.opened)
	}()
	if block {
		select {
		case <-f.opened:
		case <-f.closed:
			return nil, f.err
		}
	}
	return f, nil
}

// Read from a fifo to a byte array.
func (f *fifo) Read(b []byte) (int, error) {
	if f.flag&syscall.O_WRONLY > 0 {
		return 0, ErrRdFrmWRONLY
	}
	select {
	case <-f.opened:
		return f.file.Read(b)
	default:
	}
	select {
	case <-f.opened:
		return f.file.Read(b)
	case <-f.closed:
		return 0, ErrReadClosed
	}
}

// Write from byte array to a fifo.
func (f *fifo) Write(b []byte) (int, error) {
	if f.flag&(syscall.O_WRONLY|syscall.O_RDWR) == 0 {
		return 0, ErrWrToRDONLY
	}
	select {
	case <-f.opened:
		return f.file.Write(b)
	default:
	}
	select {
	case <-f.opened:
		return f.file.Write(b)
	case <-f.closed:
		return 0, ErrWriteClosed
	}
}

// Close the fifo. Next reads/writes will error. This method can also be used
// before open(2) has returned and fifo was never opened.
func (f *fifo) Close() (retErr error) {
	for {
		select {
		case <-f.closed:
			f.handle.Close()
			return
		default:
			select {
			case <-f.opened:
				f.closedOnce.Do(func() {
					retErr = f.file.Close()
					f.err = retErr
					close(f.closed)
				})
			default:
				if f.flag&syscall.O_RDWR != 0 {
					runtime.Gosched()
					break
				}
				f.closingOnce.Do(func() {
					close(f.closing)
				})
				reverseMode := syscall.O_WRONLY
				if f.flag&syscall.O_WRONLY > 0 {
					reverseMode = syscall.O_RDONLY
				}
				fn, err := f.handle.Path()
				// if Close() is called concurrently(shouldn't) it may cause error
				// because handle is closed
				select {
				case <-f.closed:
				default:
					if err != nil {
						// Path has become invalid. We will leak a goroutine.
						// This case should not happen in linux.
						f.closedOnce.Do(func() {
							f.err = err
							close(f.closed)
						})
						<-f.closed
						break
					}
					f, err := os.OpenFile(fn, reverseMode|syscall.O_NONBLOCK, 0)
					if err == nil {
						f.Close()
					}
					runtime.Gosched()
				}
			}
		}
	}
}

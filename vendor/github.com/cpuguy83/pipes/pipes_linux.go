package pipes

import (
	"os"

	"golang.org/x/sys/unix"
)

// New creates a pipe with a read and a write end.
// Writes on one end are met with reads on the other.
//
// This uses pipe2(2) to create the pipe.
func New() (*PipeReader, *PipeWriter, error) {
	var p [2]int
	if err := unix.Pipe2(p[:], unix.O_CLOEXEC|unix.O_NONBLOCK); err != nil {
		return nil, nil, err
	}
	pr := &PipeReader{fd: os.NewFile(uintptr(p[0]), "read")}
	pw := &PipeWriter{fd: os.NewFile(uintptr(p[1]), "write")}
	return pr, pw, nil
}

// Open opens a fifo in read only mode
// It *does not* block when opening.
// This should have smiliar semantics to os.Open, except this is for a fifo.
//
// See OpenFifo more more granular control.
func Open(p string) (*PipeReader, error) {
	pr, _, err := OpenFifo(p, os.O_RDONLY, 0)
	return pr, err
}

// Create opens the fifo with RDWR mode. If the fifo does not exist it will
// create it with 0666 (before umask) permissions.
//
// This should have similar semnatics to os.Create, except for fifos.
func Create(p string) (*PipeReader, *PipeWriter, error) {
	return OpenFifo(p, os.O_RDWR|os.O_CREATE, 0666)
}

// OpenFifoResult is used by AsyncOpenFifo to send the results of OpenFifo to a
// caller.
type OpenFifoResult struct {
	R   *PipeReader
	W   *PipeWriter
	Err error
}

// AsyncOpenFifo opens the fifo in a goroutine and sends the result on a channel.
// This is usefull, for instance, if you want to open in write-only mode and the
// read side is not yet open.
//
// Note that this will create the fifo *before* returning *if* you have passed os.O_CREATE.
func AsyncOpenFifo(p string, flag int, mode os.FileMode) (<-chan OpenFifoResult, error) {
	if err := mkFifo(p, flag, mode); err != nil {
		return nil, err
	}

	ch := make(chan OpenFifoResult, 1)
	go func() {
		pr, pw, err := OpenFifo(p, flag, mode)
		ch <- OpenFifoResult{R: pr, W: pw, Err: err}
	}()
	return ch, nil
}

func mkFifo(p string, flag int, mode os.FileMode) error {
	if flag&os.O_CREATE == 0 {
		// nothing to do
		return nil
	}

	if _, err := os.Stat(p); !os.IsNotExist(err) {
		return err
	}
	return unix.Mkfifo(p, uint32(mode.Perm()))
}

// OpenFifo opens a fifo from the provided path.
// The fifo is always opened in non-blocking mode.
//
// If flag includes os.O_CREATE this will create the fifo.
// The mode parameter should be used to set fifo permissions.
//
// Note, according to Linux fifo semantics, this will block if you are trying
// to open as os.O_WRONLY and nothing has the fifo opened in read-mode. To
// ensure a non-blocking experience, use os.O_RDWR.
// You can open once with RDWR, then open again with O_WRONLY to get around
// this semantic.
//
// If no open mode is specified (RDWR, RDONLY, WRONLY), then RDWR is used.
func OpenFifo(p string, flag int, mode os.FileMode) (pr *PipeReader, pw *PipeWriter, _ error) {
	if flag&os.O_RDWR == 0 && flag&os.O_RDONLY == 0 && flag&os.O_WRONLY == 0 {
		flag |= os.O_RDWR
	}

	if err := mkFifo(p, flag, mode); err != nil {
		return nil, nil, err
	}

	flag &= ^os.O_CREATE

	f, err := os.OpenFile(p, flag, 0)
	if err != nil {
		return nil, nil, err
	}

	var maybeDup bool
	if flag&os.O_RDONLY != 0 || flag&os.O_RDWR != 0 {
		maybeDup = true
		pr = &PipeReader{fd: f}
	}
	if flag&os.O_WRONLY != 0 || flag&os.O_RDWR != 0 {
		if maybeDup {
			// we need to dup the file descriptor so we can have two
			// file descriptors open to the same fifo.
			// This is because the underlying file descriptor is shared
			// between the two ends of the fifo.

			rc, err := f.SyscallConn()
			if err != nil {
				return nil, nil, err
			}

			var (
				dupErr error
				nfd    int
			)
			err = rc.Control(func(fd uintptr) {
				nfd, dupErr = unix.Dup(int(fd))
			})
			if err == nil && dupErr != nil {
				dupErr = os.NewSyscallError("dup", dupErr)
			}
			if err != nil {
				return nil, nil, err
			}

			f = os.NewFile(uintptr(nfd), p)

		}
		pw = &PipeWriter{fd: f}
	}
	return pr, pw, nil
}

func splice(rfd, wfd int, remain int64) (copied int64, spliceErr error) {
	noEnd := remain == 0
	if noEnd {
		remain = 1 << 62
	}

	spliceOpts := unix.SPLICE_F_MOVE | unix.SPLICE_F_NONBLOCK | unix.SPLICE_F_MORE

	for remain > 0 {
		n, err := unix.Splice(rfd, nil, wfd, nil, int(remain), spliceOpts)
		if n > 0 {
			copied += int64(n)
			if !noEnd {
				remain -= int64(n)
			}
		}

		spliceErr = err

		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return
		}

		if n == 0 {
			// EOF
			return
		}
	}

	return
}

func tee(rfd, wfd int, do int64) (copied int64, teeErr error) {
	if do == 0 {
		do = 1 << 62
	}

	// Note, this is not using SPLICE_F_NONBLOCK otherwise we'll end up getting in
	// a situation where we copied less than desired due to non-blocking writes,
	// but then with tee we can't try again because the reader side has not
	// advanced at all.
	for {
		n, err := unix.Tee(rfd, wfd, int(do), unix.SPLICE_F_MOVE)
		if err == unix.EINTR {
			continue
		}
		if n > 0 || err != nil {
			return n, err
		}
	}
}

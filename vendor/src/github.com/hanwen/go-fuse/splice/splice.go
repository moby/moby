package splice

// Routines for efficient file to file copying.

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"syscall"
)

var maxPipeSize int
var resizable bool

func Resizable() bool {
	return resizable
}

func MaxPipeSize() int {
	return maxPipeSize
}

// From manpage on ubuntu Lucid:
//
// Since Linux 2.6.11, the pipe capacity is 65536 bytes.
const DefaultPipeSize = 16 * 4096

func init() {
	content, err := ioutil.ReadFile("/proc/sys/fs/pipe-max-size")
	if err != nil {
		maxPipeSize = DefaultPipeSize
	} else {
		fmt.Sscan(string(content), &maxPipeSize)
	}

	r, w, err := os.Pipe()
	if err != nil {
		log.Panicf("cannot create pipe: %v", err)
	}
	sz, errNo := fcntl(r.Fd(), F_GETPIPE_SZ, 0)
	resizable = (errNo == 0)
	_, errNo = fcntl(r.Fd(), F_SETPIPE_SZ, 2*sz)
	resizable = resizable && (errNo == 0)
	r.Close()
	w.Close()
}

// copy & paste from syscall.
func fcntl(fd uintptr, cmd int, arg int) (val int, errno syscall.Errno) {
	r0, _, e1 := syscall.Syscall(syscall.SYS_FCNTL, fd, uintptr(cmd), uintptr(arg))
	val = int(r0)
	errno = syscall.Errno(e1)
	return
}

const F_SETPIPE_SZ = 1031
const F_GETPIPE_SZ = 1032

func newSplicePair() (p *Pair, err error) {
	p = &Pair{}
	p.r, p.w, err = os.Pipe()
	if err != nil {
		return nil, err
	}

	errNo := syscall.Errno(0)
	for _, f := range []*os.File{p.r, p.w} {
		_, errNo = fcntl(f.Fd(), syscall.F_SETFL, syscall.O_NONBLOCK)
		if errNo != 0 {
			p.Close()
			return nil, os.NewSyscallError("fcntl setfl", errNo)
		}
	}

	p.size, errNo = fcntl(p.r.Fd(), F_GETPIPE_SZ, 0)
	if errNo == syscall.EINVAL {
		p.size = DefaultPipeSize
		return p, nil
	}
	if errNo != 0 {
		p.Close()
		return nil, os.NewSyscallError("fcntl getsize", errNo)
	}
	return p, nil
}

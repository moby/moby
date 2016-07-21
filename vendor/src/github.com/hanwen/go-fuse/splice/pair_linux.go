package splice

import (
	"fmt"
	"os"
	"syscall"
)

func (p *Pair) LoadFromAt(fd uintptr, sz int, off int64) (int, error) {
	n, err := syscall.Splice(int(fd), &off, int(p.w.Fd()), nil, sz, 0)
	return int(n), err
}

func (p *Pair) LoadFrom(fd uintptr, sz int) (int, error) {
	if sz > p.size {
		return 0, fmt.Errorf("LoadFrom: not enough space %d, %d",
			sz, p.size)
	}

	n, err := syscall.Splice(int(fd), nil, int(p.w.Fd()), nil, sz, 0)
	if err != nil {
		err = os.NewSyscallError("Splice load from", err)
	}
	return int(n), err
}

func (p *Pair) WriteTo(fd uintptr, n int) (int, error) {
	m, err := syscall.Splice(int(p.r.Fd()), nil, int(fd), nil, int(n), 0)
	if err != nil {
		err = os.NewSyscallError("Splice write", err)
	}
	return int(m), err
}

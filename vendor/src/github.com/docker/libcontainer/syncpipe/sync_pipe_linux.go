package syncpipe

import (
	"os"
	"syscall"
)

func NewSyncPipe() (s *SyncPipe, err error) {
	s = &SyncPipe{}

	fds, err := syscall.Socketpair(syscall.AF_LOCAL, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}

	s.child = os.NewFile(uintptr(fds[0]), "child syncpipe")
	s.parent = os.NewFile(uintptr(fds[1]), "parent syncpipe")

	return s, nil
}

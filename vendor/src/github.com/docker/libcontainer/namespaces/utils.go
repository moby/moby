// +build linux

package namespaces

import (
	"os"
	"syscall"
)

type initError struct {
	Message string `json:"message,omitempty"`
}

func (i initError) Error() string {
	return i.Message
}

// New returns a newly initialized Pipe for communication between processes
func newInitPipe() (parent *os.File, child *os.File, err error) {
	fds, err := syscall.Socketpair(syscall.AF_LOCAL, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	return os.NewFile(uintptr(fds[1]), "parent"), os.NewFile(uintptr(fds[0]), "child"), nil
}

// GetNamespaceFlags parses the container's Namespaces options to set the correct
// flags on clone, unshare, and setns
func GetNamespaceFlags(namespaces map[string]bool) (flag int) {
	for key, enabled := range namespaces {
		if enabled {
			if ns := GetNamespace(key); ns != nil {
				flag |= ns.Value
			}
		}
	}
	return flag
}

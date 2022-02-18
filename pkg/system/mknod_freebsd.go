//go:build freebsd
// +build freebsd

package system // import "github.com/moby/moby/pkg/system"

import (
	"golang.org/x/sys/unix"
)

// Mknod creates a filesystem node (file, device special file or named pipe) named path
// with attributes specified by mode and dev.
func Mknod(path string, mode uint32, dev int) error {
	return unix.Mknod(path, mode, uint64(dev))
}

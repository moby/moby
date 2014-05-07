// +build linux

package mount

import (
	"github.com/dotcloud/docker/pkg/system"
	"syscall"
)

func SetReadonly() error {
	return system.Mount("/", "/", "bind", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_REC, "")
}

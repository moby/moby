// +build linux

package mount

import (
	"syscall"

	"github.com/dotcloud/docker/pkg/system"
)

func SetReadonly() error {
	return system.Mount("/", "/", "bind", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_REC, "")
}

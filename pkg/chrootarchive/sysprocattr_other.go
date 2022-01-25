//go:build !linux
// +build !linux

package chrootarchive // import "github.com/docker/docker/pkg/chrootarchive"

import "syscall"

func sysProcAttr() *syscall.SysProcAttr {
	return nil
}

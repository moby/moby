package supervisor // import "github.com/docker/docker/libcontainerd/supervisor"

import "syscall"

func containerdSysProcAttr() *syscall.SysProcAttr {
	return nil
}

package ipc

import (
	"fmt"
	"os"
	"syscall"

	"github.com/docker/libcontainer/system"
)

// Join the IPC Namespace of specified ipc path if it exists.
// If the path does not exist then you are not joining a container.
func Initialize(nsPath string) error {
	if nsPath == "" {
		return nil
	}
	f, err := os.OpenFile(nsPath, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed get IPC namespace fd: %v", err)
	}

	err = system.Setns(f.Fd(), syscall.CLONE_NEWIPC)
	f.Close()

	if err != nil {
		return fmt.Errorf("failed to setns current IPC namespace: %v", err)
	}
	return nil
}

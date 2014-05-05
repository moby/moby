// +build linux

package nsinit

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/dotcloud/docker/pkg/label"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/mount"
	"github.com/dotcloud/docker/pkg/system"
)

// ExecIn uses an existing pid and joins the pid's namespaces with the new command.
func ExecIn(container *libcontainer.Container, nspid int, args []string) (int, error) {
	// clear the current processes env and replace it with the environment
	// defined on the container
	if err := LoadContainerEnvironment(container); err != nil {
		return -1, err
	}

	for key, enabled := range container.Namespaces {
		// skip the PID namespace on unshare because it it not supported
		if enabled && key != "NEWPID" {
			if ns := libcontainer.GetNamespace(key); ns != nil {
				if err := system.Unshare(ns.Value); err != nil {
					return -1, err
				}
			}
		}
	}
	fds, err := getNsFds(nspid, container)
	closeFds := func() {
		for _, f := range fds {
			system.Closefd(f)
		}
	}
	if err != nil {
		closeFds()
		return -1, err
	}
	processLabel, err := label.GetPidCon(nspid)
	if err != nil {
		closeFds()
		return -1, err
	}
	// foreach namespace fd, use setns to join an existing container's namespaces
	for _, fd := range fds {
		if fd > 0 {
			if err := system.Setns(fd, 0); err != nil {
				closeFds()
				return -1, fmt.Errorf("setns %s", err)
			}
		}
		system.Closefd(fd)
	}

	// if the container has a new pid and mount namespace we need to
	// remount proc and sys to pick up the changes
	if container.Namespaces["NEWNS"] && container.Namespaces["NEWPID"] {
		pid, err := system.Fork()
		if err != nil {
			return -1, err
		}
		if pid == 0 {
			// TODO: make all raw syscalls to be fork safe
			if err := system.Unshare(syscall.CLONE_NEWNS); err != nil {
				return -1, err
			}
			if err := mount.RemountProc(); err != nil {
				return -1, fmt.Errorf("remount proc %s", err)
			}
			if err := mount.RemountSys(); err != nil {
				return -1, fmt.Errorf("remount sys %s", err)
			}
			goto dropAndExec
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			return -1, err
		}
		state, err := proc.Wait()
		if err != nil {
			return -1, err
		}
		os.Exit(state.Sys().(syscall.WaitStatus).ExitStatus())
	}
dropAndExec:
	if err := FinalizeNamespace(container); err != nil {
		return -1, err
	}
	err = label.SetProcessLabel(processLabel)
	if err != nil {
		return -1, err
	}
	if err := system.Execv(args[0], args[0:], container.Env); err != nil {
		return -1, err
	}
	panic("unreachable")
}

func getNsFds(pid int, container *libcontainer.Container) ([]uintptr, error) {
	fds := []uintptr{}

	for key, enabled := range container.Namespaces {
		if enabled {
			if ns := libcontainer.GetNamespace(key); ns != nil {
				f, err := os.OpenFile(filepath.Join("/proc/", strconv.Itoa(pid), "ns", ns.File), os.O_RDONLY, 0)
				if err != nil {
					return fds, err
				}
				fds = append(fds, f.Fd())
			}
		}
	}
	return fds, nil
}

/*
   Higher level convience functions for setting up a container
*/

package namespaces

import (
	"errors"
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/libcontainer/capabilities"
	"github.com/dotcloud/docker/pkg/libcontainer/utils"
	"io"
	"log"
	"os"
	"path/filepath"
	"syscall"
)

var (
	ErrExistingNetworkNamespace = errors.New("specified both CLONE_NEWNET and an existing network namespace")
)

// Exec will spawn new namespaces with the specified Container configuration
// in the RootFs path and return the pid of the new containerized process.
//
// If an existing network namespace is specified the container
// will join that namespace.  If an existing network namespace is not specified but CLONE_NEWNET is,
// the container will be spawned with a new network namespace with no configuration.  Omiting an
// existing network namespace and the CLONE_NEWNET option in the container configuration will allow
// the container to the the host's networking options and configuration.
func Exec(container *libcontainer.Container) (pid int, err error) {
	// a user cannot pass CLONE_NEWNET and an existing net namespace fd to join
	if container.NetNsFd > 0 && container.Namespaces.Contains(libcontainer.CLONE_NEWNET) {
		return -1, ErrExistingNetworkNamespace
	}

	rootfs, err := resolveRootfs(container)
	if err != nil {
		return -1, err
	}

	master, console, err := createMasterAndConsole()
	if err != nil {
		return -1, err
	}

	logger, err := os.OpenFile("/root/logs", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		return -1, err
	}
	log.SetOutput(logger)

	// we need CLONE_VFORK so we can wait on the child
	flag := getNamespaceFlags(container.Namespaces) | CLONE_VFORK

	if pid, err = clone(uintptr(flag | SIGCHLD)); err != nil {
		return -1, fmt.Errorf("error cloning process: %s", err)
	}

	if pid == 0 {
		// welcome to your new namespace ;)
		//
		// any errors encoutered inside the namespace we should write
		// out to a log or a pipe to our parent and exit(1)
		// because writing to stderr will not work after we close
		if err := closeMasterAndStd(master); err != nil {
			writeError("close master and std %s", err)
		}
		slave, err := openTerminal(console, syscall.O_RDWR)
		if err != nil {
			writeError("open terminal %s", err)
		}
		if err := dupSlave(slave); err != nil {
			writeError("dup2 slave %s", err)
		}

		if container.NetNsFd > 0 {
			if err := JoinExistingNamespace(container.NetNsFd, libcontainer.CLONE_NEWNET); err != nil {
				writeError("join existing net namespace %s", err)
			}
		}

		if _, err := setsid(); err != nil {
			writeError("setsid %s", err)
		}
		if err := setctty(); err != nil {
			writeError("setctty %s", err)
		}
		if err := parentDeathSignal(); err != nil {
			writeError("parent deth signal %s", err)
		}
		if err := SetupNewMountNamespace(rootfs, console, container.ReadonlyFs); err != nil {
			writeError("setup mount namespace %s", err)
		}
		if err := sethostname(container.ID); err != nil {
			writeError("sethostname %s", err)
		}
		if err := capabilities.DropCapabilities(container); err != nil {
			writeError("drop capabilities %s", err)
		}
		if err := setupUser(container); err != nil {
			writeError("setup user %s", err)
		}
		if container.WorkingDir != "" {
			if err := chdir(container.WorkingDir); err != nil {
				writeError("chdir to %s %s", container.WorkingDir, err)
			}
		}
		if err := exec(container.Command.Args[0], container.Command.Args[0:], container.Command.Env); err != nil {
			writeError("exec %s", err)
		}
		panic("unreachable")
	}

	go func() {
		if _, err := io.Copy(os.Stdout, master); err != nil {
			log.Println(err)
		}
	}()
	go func() {
		if _, err := io.Copy(master, os.Stdin); err != nil {
			log.Println(err)
		}
	}()
	return pid, nil
}

// ExecIn will spawn a new command inside an existing container's namespaces.  The existing container's
// pid and namespace configuration is needed along with the specific capabilities that should
// be dropped once inside the namespace.
func ExecIn(container *libcontainer.Container, cmd *libcontainer.Command) (int, error) {
	if container.NsPid <= 0 {
		return -1, libcontainer.ErrInvalidPid
	}

	fds, err := getNsFds(container)
	if err != nil {
		return -1, err
	}

	if container.NetNsFd > 0 {
		fds = append(fds, container.NetNsFd)
	}

	pid, err := fork()
	if err != nil {
		for _, fd := range fds {
			syscall.Close(int(fd))
		}
		return -1, err
	}

	if pid == 0 {
		for _, fd := range fds {
			if fd > 0 {
				if err := JoinExistingNamespace(fd, ""); err != nil {
					for _, fd := range fds {
						syscall.Close(int(fd))
					}
					writeError("join existing namespace for %d %s", fd, err)
				}
			}
			syscall.Close(int(fd))
		}

		if container.Namespaces.Contains(libcontainer.CLONE_NEWNS) &&
			container.Namespaces.Contains(libcontainer.CLONE_NEWPID) {
			// important:
			//
			// we need to fork and unshare so that re can remount proc and sys within
			// the namespace so the CLONE_NEWPID namespace will take effect
			// if we don't fork we would end up unmounting proc and sys for the entire
			// namespace
			child, err := fork()
			if err != nil {
				writeError("fork child %s", err)
			}

			if child == 0 {
				if err := unshare(CLONE_NEWNS); err != nil {
					writeError("unshare newns %s", err)
				}
				if err := remountProc(); err != nil {
					writeError("remount proc %s", err)
				}
				if err := remountSys(); err != nil {
					writeError("remount sys %s", err)
				}
				if err := capabilities.DropCapabilities(container); err != nil {
					writeError("drop caps %s", err)
				}
				if err := exec(cmd.Args[0], cmd.Args[0:], cmd.Env); err != nil {
					writeError("exec %s", err)
				}
				panic("unreachable")
			}
			exit, err := utils.WaitOnPid(child)
			if err != nil {
				writeError("wait on child %s", err)
			}
			os.Exit(exit)
		}
		if err := exec(cmd.Args[0], cmd.Args[0:], cmd.Env); err != nil {
			writeError("exec %s", err)
		}
		panic("unreachable")
	}
	return pid, err
}

func resolveRootfs(container *libcontainer.Container) (string, error) {
	rootfs, err := filepath.Abs(container.RootFs)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(rootfs)
}

func createMasterAndConsole() (*os.File, string, error) {
	master, err := os.OpenFile("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, "", err
	}

	console, err := ptsname(master)
	if err != nil {
		return nil, "", err
	}

	if err := unlockpt(master); err != nil {
		return nil, "", err
	}
	return master, console, nil
}

func closeMasterAndStd(master *os.File) error {
	closefd(master.Fd())
	closefd(0)
	closefd(1)
	closefd(2)

	return nil
}

func dupSlave(slave *os.File) error {
	// we close Stdin,etc so our pty slave should have fd 0
	if slave.Fd() != 0 {
		return fmt.Errorf("slave fd not 0 %d", slave.Fd())
	}
	if err := dup2(slave.Fd(), 1); err != nil {
		return err
	}
	if err := dup2(slave.Fd(), 2); err != nil {
		return err
	}
	return nil
}

func openTerminal(name string, flag int) (*os.File, error) {
	r, e := syscall.Open(name, flag, 0)
	if e != nil {
		return nil, &os.PathError{"open", name, e}
	}
	return os.NewFile(uintptr(r), name), nil
}

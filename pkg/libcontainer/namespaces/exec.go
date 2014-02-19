/*
   Higher level convience functions for setting up a container
*/

package namespaces

import (
	"errors"
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"io"
	"log"
	"os"
	"os/exec"
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
func ExecContainer(container *libcontainer.Container) (pid int, err error) {
	// a user cannot pass CLONE_NEWNET and an existing net namespace fd to join
	if container.NetNsFd > 0 && container.Namespaces.Contains(libcontainer.CLONE_NEWNET) {
		return -1, ErrExistingNetworkNamespace
	}

	master, console, err := createMasterAndConsole()
	if err != nil {
		return -1, err
	}
	container.Console = console
	container.Master = master.Fd()

	// we need CLONE_VFORK so we can wait on the child
	flag := uintptr(getNamespaceFlags(container.Namespaces) | CLONE_VFORK)

	command := exec.Command("/.nsinit")
	command.SysProcAttr = &syscall.SysProcAttr{}
	command.SysProcAttr.Cloneflags = flag
	command.SysProcAttr.Setctty = true

	if err := command.Start(); err != nil {
		return -1, err
	}
	pid = command.Process.Pid

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
	return -1, fmt.Errorf("not implemented")
	/*
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
	*/
}

func createMasterAndConsole() (*os.File, string, error) {
	master, err := os.OpenFile("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, "", err
	}

	console, err := Ptsname(master)
	if err != nil {
		return nil, "", err
	}

	if err := Unlockpt(master); err != nil {
		return nil, "", err
	}
	return master, console, nil
}

/*
   Higher level convience functions for setting up a container
*/

package namespaces

import (
	"errors"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/system"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	nsinit := filepath.Join(container.RootFs, ".nsinit")

	// we need CLONE_VFORK so we can wait on the child
	flag := uintptr(getNamespaceFlags(container.Namespaces) | CLONE_VFORK)

	command := exec.Command(nsinit, "-master", strconv.Itoa(int(master.Fd())), "-console", console, "init", "container.json")
	// command.Stdin = os.Stdin
	// command.Stdout = os.Stdout
	// command.Stderr = os.Stderr
	command.SysProcAttr = &syscall.SysProcAttr{}
	command.SysProcAttr.Cloneflags = flag
	//command.ExtraFiles = []*os.File{master}

	println("vvvvvvvvv")
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

func createMasterAndConsole() (*os.File, string, error) {
	master, err := os.OpenFile("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, "", err
	}

	console, err := system.Ptsname(master)
	if err != nil {
		return nil, "", err
	}

	if err := system.Unlockpt(master); err != nil {
		return nil, "", err
	}
	return master, console, nil
}

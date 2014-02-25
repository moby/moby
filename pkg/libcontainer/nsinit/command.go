package nsinit

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/libcontainer"
	"os"
	"os/exec"
	"syscall"
)

// CommandFactory takes the container's configuration and options passed by the
// parent processes and creates an *exec.Cmd that will be used to fork/exec the
// namespaced init process
type CommandFactory interface {
	Create(container *libcontainer.Container, console string, syncFd uintptr, args []string) *exec.Cmd
}

type DefaultCommandFactory struct{}

// Create will return an exec.Cmd with the Cloneflags set to the proper namespaces
// defined on the container's configuration and use the current binary as the init with the
// args provided
func (c *DefaultCommandFactory) Create(container *libcontainer.Container, console string, pipe uintptr, args []string) *exec.Cmd {
	// get our binary name so we can always reexec ourself
	name := os.Args[0]
	command := exec.Command(name, append([]string{
		"-console", console,
		"-pipe", fmt.Sprint(pipe),
		"init"}, args...)...)

	command.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: uintptr(GetNamespaceFlags(container.Namespaces)),
	}
	command.Env = container.Env
	return command
}

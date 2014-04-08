package nsinit

import (
	"github.com/dotcloud/docker/pkg/libcontainer"
	"github.com/dotcloud/docker/pkg/system"
	"os"
	"os/exec"
)

// CommandFactory takes the container's configuration and options passed by the
// parent processes and creates an *exec.Cmd that will be used to fork/exec the
// namespaced init process
type CommandFactory interface {
	Create(container *libcontainer.Container, console string, syncFd *os.File, args []string) *exec.Cmd
}

type DefaultCommandFactory struct {
	Root string
}

// Create will return an exec.Cmd with the Cloneflags set to the proper namespaces
// defined on the container's configuration and use the current binary as the init with the
// args provided
func (c *DefaultCommandFactory) Create(container *libcontainer.Container, console string, pipe *os.File, args []string) *exec.Cmd {
	// get our binary name from arg0 so we can always reexec ourself
	command := exec.Command(os.Args[0], append([]string{
		"-console", console,
		"-pipe", "3",
		"-root", c.Root,
		"init"}, args...)...)

	system.SetCloneFlags(command, uintptr(GetNamespaceFlags(container.Namespaces)))
	command.Env = container.Env
	command.ExtraFiles = []*os.File{pipe}
	return command
}

// GetNamespaceFlags parses the container's Namespaces options to set the correct
// flags on clone, unshare, and setns
func GetNamespaceFlags(namespaces libcontainer.Namespaces) (flag int) {
	for _, ns := range namespaces {
		if ns.Enabled {
			flag |= ns.Value
		}
	}
	return flag
}

// +build linux

package native

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/namespaces"
)

const execCommandName = "nsenter-exec"

func init() {
	reexec.Register(execCommandName, nsenterExec)
}

func nsenterExec() {
	runtime.LockOSThread()

	// User args are passed after '--' in the command line.
	userArgs := findUserArgs()

	config, err := loadConfigFromFd()
	if err != nil {
		log.Fatalf("docker-exec: unable to receive config from sync pipe: %s", err)
	}

	if err := namespaces.FinalizeSetns(config, userArgs); err != nil {
		log.Fatalf("docker-exec: failed to exec: %s", err)
	}
}

// TODO(vishh): Add support for running in priviledged mode and running as a different user.
func (d *driver) Exec(c *execdriver.Command, processConfig *execdriver.ProcessConfig, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (int, error) {
	active := d.activeContainers[c.ID]
	if active == nil {
		return -1, fmt.Errorf("No active container exists with ID %s", c.ID)
	}
	state, err := libcontainer.GetState(filepath.Join(d.root, c.ID))
	if err != nil {
		return -1, fmt.Errorf("State unavailable for container with ID %s. The container may have been cleaned up already. Error: %s", c.ID, err)
	}

	var term execdriver.Terminal

	if processConfig.Tty {
		term, err = NewTtyConsole(processConfig, pipes)
	} else {
		term, err = execdriver.NewStdConsole(processConfig, pipes)
	}

	processConfig.Terminal = term

	args := append([]string{processConfig.Entrypoint}, processConfig.Arguments...)

	return namespaces.ExecIn(active.container, state, args, os.Args[0], "exec", processConfig.Stdin, processConfig.Stdout, processConfig.Stderr, processConfig.Console,
		func(cmd *exec.Cmd) {
			if startCallback != nil {
				startCallback(&c.ProcessConfig, cmd.Process.Pid)
			}
		})
}

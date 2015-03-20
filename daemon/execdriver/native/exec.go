// +build linux

package native

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/libcontainer"
	_ "github.com/docker/libcontainer/nsenter"
	"github.com/docker/libcontainer/utils"
)

// TODO(vishh): Add support for running in priviledged mode and running as a different user.
func (d *driver) Exec(c *execdriver.Command, processConfig *execdriver.ProcessConfig, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (int, error) {
	active := d.activeContainers[c.ID]
	if active == nil {
		return -1, fmt.Errorf("No active container exists with ID %s", c.ID)
	}

	var term execdriver.Terminal
	var err error

	p := &libcontainer.Process{
		Args: append([]string{processConfig.Entrypoint}, processConfig.Arguments...),
		Env:  c.ProcessConfig.Env,
		Cwd:  c.WorkingDir,
		User: c.ProcessConfig.User,
	}

	if processConfig.Tty {
		config := active.Config()
		rootuid, err := config.HostUID()
		if err != nil {
			return -1, err
		}
		cons, err := p.NewConsole(rootuid)
		if err != nil {
			return -1, err
		}
		term, err = NewTtyConsole(cons, pipes, rootuid)
	} else {
		p.Stdout = pipes.Stdout
		p.Stderr = pipes.Stderr
		p.Stdin = pipes.Stdin
		term = &execdriver.StdConsole{}
	}
	if err != nil {
		return -1, err
	}

	processConfig.Terminal = term

	if err := active.Start(p); err != nil {
		return -1, err
	}

	if startCallback != nil {
		pid, err := p.Pid()
		if err != nil {
			p.Signal(os.Kill)
			p.Wait()
			return -1, err
		}
		startCallback(&c.ProcessConfig, pid)
	}

	ps, err := p.Wait()
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			return -1, err
		}
		ps = exitErr.ProcessState
	}
	return utils.ExitStatus(ps.Sys().(syscall.WaitStatus)), nil
}

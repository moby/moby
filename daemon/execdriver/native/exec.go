// +build linux

package native

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/opencontainers/runc/libcontainer"
	// Blank import 'nsenter' so that init in that package will call c
	// function 'nsexec()' to do 'setns' before Go runtime take over,
	// it's used for join to exist ns like 'docker exec' command.
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
	"github.com/opencontainers/runc/libcontainer/utils"
)

// Exec implements the exec driver Driver interface,
// it calls libcontainer APIs to execute a container.
func (d *Driver) Exec(c *execdriver.Command, processConfig *execdriver.ProcessConfig, pipes *execdriver.Pipes, hooks execdriver.Hooks) (int, error) {
	active := d.activeContainers[c.ID]
	if active == nil {
		return -1, fmt.Errorf("No active container exists with ID %s", c.ID)
	}

	user := processConfig.User
	if c.RemappedRoot.UID != 0 && user == "" {
		//if user namespaces are enabled, set user explicitly so uid/gid is set to 0
		//otherwise we end up with the overflow id and no permissions (65534)
		user = "0"
	}

	p := &libcontainer.Process{
		Args: append([]string{processConfig.Entrypoint}, processConfig.Arguments...),
		Env:  c.ProcessConfig.Env,
		Cwd:  c.WorkingDir,
		User: user,
	}

	if processConfig.Privileged {
		p.Capabilities = execdriver.GetAllCapabilities()
	}
	// add CAP_ prefix to all caps for new libcontainer update to match
	// the spec format.
	for i, s := range p.Capabilities {
		if !strings.HasPrefix(s, "CAP_") {
			p.Capabilities[i] = fmt.Sprintf("CAP_%s", s)
		}
	}

	config := active.Config()
	if err := setupPipes(&config, processConfig, p, pipes); err != nil {
		return -1, err
	}

	if err := active.Start(p); err != nil {
		return -1, err
	}

	if hooks.Start != nil {
		pid, err := p.Pid()
		if err != nil {
			p.Signal(os.Kill)
			p.Wait()
			return -1, err
		}

		// A closed channel for OOM is returned here as it will be
		// non-blocking and return the correct result when read.
		chOOM := make(chan struct{})
		close(chOOM)
		hooks.Start(&c.ProcessConfig, pid, chOOM)
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

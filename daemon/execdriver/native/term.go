/*
   These types are wrappers around the libcontainer Terminal interface so that
   we can resuse the docker implementations where possible.
*/
package native

import (
	"github.com/dotcloud/docker/daemon/execdriver"
	"io"
	"os"
	"os/exec"
)

type dockerStdTerm struct {
	execdriver.StdConsole
	pipes *execdriver.Pipes
}

func (d *dockerStdTerm) Attach(cmd *exec.Cmd) error {
	return d.AttachPipes(cmd, d.pipes)
}

func (d *dockerStdTerm) SetMaster(master *os.File) {
	// do nothing
}

type dockerTtyTerm struct {
	execdriver.TtyConsole
	pipes *execdriver.Pipes
}

func (t *dockerTtyTerm) Attach(cmd *exec.Cmd) error {
	go io.Copy(t.pipes.Stdout, t.MasterPty)
	if t.pipes.Stdin != nil {
		go io.Copy(t.MasterPty, t.pipes.Stdin)
	}
	return nil
}

func (t *dockerTtyTerm) SetMaster(master *os.File) {
	t.MasterPty = master
}

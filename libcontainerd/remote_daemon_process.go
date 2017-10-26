// +build !windows

package libcontainerd

import "github.com/pkg/errors"

// process represents the state for the main container process or an exec.
type process struct {
	// id is the logical name of the process
	id string

	// cid is the container id to which this process belongs
	cid string

	// pid is the identifier of the process
	pid uint32

	// io holds the io reader/writer associated with the process
	io *IOPipe

	// root is the state directory for the process
	root string
}

func (p *process) ID() string {
	return p.id
}

func (p *process) Pid() uint32 {
	return p.pid
}

func (p *process) SetPid(pid uint32) error {
	if p.pid != 0 {
		return errors.Errorf("pid is already set to %d", pid)
	}

	p.pid = pid
	return nil
}

func (p *process) IOPipe() *IOPipe {
	return p.io
}

func (p *process) CloseIO() {
	if p.io.Stdin != nil {
		p.io.Stdin.Close()
	}
	if p.io.Stdout != nil {
		p.io.Stdout.Close()
	}
	if p.io.Stderr != nil {
		p.io.Stderr.Close()
	}
}

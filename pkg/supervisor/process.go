package supervisor

import (
	"github.com/dotcloud/docker/pkg/system"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type Process struct {
	cmd     *exec.Cmd
	stopped bool
}

func NewProcess(args, env []string, attachStd bool) *Process {
	cmd := exec.Command(args[0], args[1:]...)
	if attachStd {
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
	}
	cmd.Env = env

	return &Process{
		cmd: cmd,
	}
}

func (p *Process) Start() error {
	return p.cmd.Start()
}

func (p *Process) Pid() int {
	return p.cmd.Process.Pid
}

func (p *Process) Kill() error {
	if p.stopped {
		return nil
	}
	err := p.cmd.Process.Kill()
	p.stopped = true
	return err
}

func (p *Process) ExitCode() int {
	return system.GetExitCode(p.cmd)
}

func (p *Process) Signal(sig os.Signal) error {
	return p.cmd.Process.Signal(sig)
}

func (p *Process) Wait() error {
	if err := p.cmd.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return err
		}
	}
	return nil
}

func (p *Process) Reap() error {
	if p.stopped {
		return nil
	}
	err := p.Kill()

	for {
		pid, werr := syscall.Wait4(-1, nil, syscall.WNOHANG, nil)
		if werr != nil {
			switch werr {
			case syscall.EAGAIN:
				time.Sleep(10 * time.Millisecond)
				continue
			default:
				if err == nil {
					err = werr
				}
			}
		}
		if werr == nil && pid == p.Pid() {
			break
		}
	}
	return err
}

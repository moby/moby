package runc

import (
	"os/exec"
	"syscall"
	"time"
)

var Monitor ProcessMonitor = &defaultMonitor{}

type Exit struct {
	Timestamp time.Time
	Pid       int
	Status    int
}

// ProcessMonitor is an interface for process monitoring
//
// It allows daemons using go-runc to have a SIGCHLD handler
// to handle exits without introducing races between the handler
// and go's exec.Cmd
// These methods should match the methods exposed by exec.Cmd to provide
// a consistent experience for the caller
type ProcessMonitor interface {
	Start(*exec.Cmd) (chan Exit, error)
	Wait(*exec.Cmd, chan Exit) (int, error)
}

type defaultMonitor struct {
}

func (m *defaultMonitor) Start(c *exec.Cmd) (chan Exit, error) {
	if err := c.Start(); err != nil {
		return nil, err
	}
	ec := make(chan Exit, 1)
	go func() {
		var status int
		if err := c.Wait(); err != nil {
			status = 255
			if exitErr, ok := err.(*exec.ExitError); ok {
				if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					status = ws.ExitStatus()
				}
			}
		}
		ec <- Exit{
			Timestamp: time.Now(),
			Pid:       c.Process.Pid,
			Status:    status,
		}
		close(ec)
	}()
	return ec, nil
}

func (m *defaultMonitor) Wait(c *exec.Cmd, ec chan Exit) (int, error) {
	e := <-ec
	return e.Status, nil
}

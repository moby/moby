/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package runc

import (
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

// Monitor is the default ProcessMonitor for handling runc process exit
var Monitor ProcessMonitor = &defaultMonitor{}

// Exit holds the exit information from a process
type Exit struct {
	Timestamp time.Time
	Pid       int
	Status    int
}

// ProcessMonitor is an interface for process monitoring.
//
// It allows daemons using go-runc to have a SIGCHLD handler
// to handle exits without introducing races between the handler
// and go's exec.Cmd.
//
// ProcessMonitor also provides a StartLocked method which is similar to
// Start, but locks the goroutine used to start the process to an OS thread
// (for example: when Pdeathsig is set).
type ProcessMonitor interface {
	Start(*exec.Cmd) (chan Exit, error)
	StartLocked(*exec.Cmd) (chan Exit, error)
	Wait(*exec.Cmd, chan Exit) (int, error)
}

type defaultMonitor struct{}

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

// StartLocked is like Start, but locks the goroutine used to start the process to
// the OS thread for use-cases where the parent thread matters to the child process
// (for example: when Pdeathsig is set).
func (m *defaultMonitor) StartLocked(c *exec.Cmd) (chan Exit, error) {
	started := make(chan error)
	ec := make(chan Exit, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		if err := c.Start(); err != nil {
			started <- err
			return
		}
		close(started)
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
	if err := <-started; err != nil {
		return nil, err
	}
	return ec, nil
}

func (m *defaultMonitor) Wait(c *exec.Cmd, ec chan Exit) (int, error) {
	e := <-ec
	return e.Status, nil
}

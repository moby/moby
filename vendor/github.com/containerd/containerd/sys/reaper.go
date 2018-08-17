// +build !windows

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

package sys

import (
	"golang.org/x/sys/unix"
)

// Exit is the wait4 information from an exited process
type Exit struct {
	Pid    int
	Status int
}

// Reap reaps all child processes for the calling process and returns their
// exit information
func Reap(wait bool) (exits []Exit, err error) {
	var (
		ws  unix.WaitStatus
		rus unix.Rusage
	)
	flag := unix.WNOHANG
	if wait {
		flag = 0
	}
	for {
		pid, err := unix.Wait4(-1, &ws, flag, &rus)
		if err != nil {
			if err == unix.ECHILD {
				return exits, nil
			}
			return exits, err
		}
		if pid <= 0 {
			return exits, nil
		}
		exits = append(exits, Exit{
			Pid:    pid,
			Status: exitStatus(ws),
		})
	}
}

const exitSignalOffset = 128

// exitStatus returns the correct exit status for a process based on if it
// was signaled or exited cleanly
func exitStatus(status unix.WaitStatus) int {
	if status.Signaled() {
		return exitSignalOffset + int(status.Signal())
	}
	return status.ExitStatus()
}

//go:build !windows

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
	"os"
)

// NewPidfdSocket creates a new unix socket at the provided path to accept the
// pidfd of the process runc creates.
//
// Requires runc v1.2.0 or newer and a v5.3 or newer kernel.
func NewPidfdSocket(path string) (*Socket, error) {
	return newSocket(path)
}

// NewTempPidfdSocket returns a temp socket to accept the pidfd of the process
// runc creates. On Close(), the socket is deleted.
//
// Requires runc v1.2.0 or newer and a v5.3 or newer kernel.
func NewTempPidfdSocket() (*Socket, error) {
	return newTempSocket("pidfd", "pidfd.sock")
}

// ReceivePidfd blocks until the socket receives the pidfd of the process runc
// created. The pidfd refers to the container's init process for create and run,
// and to the exec'd process for exec.
//
// The pidfd is sent while the process is being set up, which is before `runc
// create` returns and before the container's start fifo is opened, so it is
// safe to receive it before starting the container.
//
// It is the caller's responsibility to close the returned file. The pidfd can
// be used to signal or wait on the process without the pid reuse races that
// come with a raw pid, e.g. with unix.PidfdSendSignal.
func (c *Socket) ReceivePidfd() (*os.File, error) {
	return c.receive()
}

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
	"github.com/containerd/console"
)

// NewConsoleSocket creates a new unix socket at the provided path to accept a
// pty master created by runc for use by the container
func NewConsoleSocket(path string) (*Socket, error) {
	return newSocket(path)
}

// NewTempConsoleSocket returns a temp console socket for use with a container
// On Close(), the socket is deleted
func NewTempConsoleSocket() (*Socket, error) {
	return newTempSocket("pty", "pty.sock")
}

// ReceiveMaster blocks until the socket receives the pty master
func (c *Socket) ReceiveMaster() (console.Console, error) {
	f, err := c.receive()
	if err != nil {
		return nil, err
	}
	return console.ConsoleFromFile(f)
}

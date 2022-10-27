// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !windows
// +build !windows

// Package activation implements primitives for systemd socket activation.
package activation

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

const (
	// listenFdsStart corresponds to `SD_LISTEN_FDS_START`.
	listenFdsStart = 3
)

// Files returns a slice containing a `os.File` object for each
// file descriptor passed to this process via systemd fd-passing protocol.
//
// The order of the file descriptors is preserved in the returned slice.
// `unsetEnv` is typically set to `true` in order to avoid clashes in
// fd usage and to avoid leaking environment flags to child processes.
func Files(unsetEnv bool) []*os.File {
	if unsetEnv {
		defer os.Unsetenv("LISTEN_PID")
		defer os.Unsetenv("LISTEN_FDS")
		defer os.Unsetenv("LISTEN_FDNAMES")
	}

	pid, err := strconv.Atoi(os.Getenv("LISTEN_PID"))
	if err != nil || pid != os.Getpid() {
		return nil
	}

	nfds, err := strconv.Atoi(os.Getenv("LISTEN_FDS"))
	if err != nil || nfds == 0 {
		return nil
	}

	names := strings.Split(os.Getenv("LISTEN_FDNAMES"), ":")

	files := make([]*os.File, 0, nfds)
	for fd := listenFdsStart; fd < listenFdsStart+nfds; fd++ {
		syscall.CloseOnExec(fd)
		name := "LISTEN_FD_" + strconv.Itoa(fd)
		offset := fd - listenFdsStart
		if offset < len(names) && len(names[offset]) > 0 {
			name = names[offset]
		}
		files = append(files, os.NewFile(uintptr(fd), name))
	}

	return files
}

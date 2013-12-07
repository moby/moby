// Package activation implements primitives for systemd socket activation.
package activation

import (
	"os"
	"strconv"
	"syscall"
)

// based on: https://gist.github.com/alberts/4640792
const (
	listenFdsStart = 3
)

func Files(unsetEnv bool) []*os.File {

	if unsetEnv {
		// there is no way to unset env in golang os package for now
		// https://code.google.com/p/go/issues/detail?id=6423
		defer os.Setenv("LISTEN_PID", "")
		defer os.Setenv("LISTEN_FDS", "")
	}

	pid, err := strconv.Atoi(os.Getenv("LISTEN_PID"))
	if err != nil || pid != os.Getpid() {
		return nil
	}
	nfds, err := strconv.Atoi(os.Getenv("LISTEN_FDS"))
	if err != nil || nfds == 0 {
		return nil
	}
	var files []*os.File
	for fd := listenFdsStart; fd < listenFdsStart+nfds; fd++ {
		syscall.CloseOnExec(fd)
		files = append(files, os.NewFile(uintptr(fd), "LISTEN_FD_"+strconv.Itoa(fd)))
	}
	return files
}

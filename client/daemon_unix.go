// +build !windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

// CmdDaemon execs dockerd with the same flags
// TODO: add a deprecation warning?
func (p DaemonProxy) CmdDaemon(args ...string) error {
	// Use os.Args[1:] so that "global" args are passed to dockerd
	args = stripDaemonArg(os.Args[1:])

	// TODO: check dirname args[0] first
	binaryAbsPath, err := exec.LookPath(daemonBinary)
	if err != nil {
		return err
	}

	return syscall.Exec(
		binaryAbsPath,
		append([]string{daemonBinary}, args...),
		os.Environ())
}

// stripDaemonArg removes the `daemon` argument from the list
func stripDaemonArg(args []string) []string {
	for i, arg := range args {
		if arg == "daemon" {
			return append(args[:i], args[i+1:]...)
		}
	}
	return args
}

// +build daemon

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// CmdDaemon execs dockerd with the same flags
func (p DaemonProxy) CmdDaemon(args ...string) error {
	// Use os.Args[1:] so that "global" args are passed to dockerd
	args = stripDaemonArg(os.Args[1:])

	binaryPath, err := findDaemonBinary()
	if err != nil {
		return err
	}

	return syscall.Exec(
		binaryPath,
		append([]string{daemonBinary}, args...),
		os.Environ())
}

// findDaemonBinary looks for the path to the dockerd binary starting with
// the directory of the current executable (if one exists) and followed by $PATH
func findDaemonBinary() (string, error) {
	execDirname := filepath.Dir(os.Args[0])
	if execDirname != "" {
		binaryPath := filepath.Join(execDirname, daemonBinary)
		if _, err := os.Stat(binaryPath); err == nil {
			return binaryPath, nil
		}
	}

	return exec.LookPath(daemonBinary)
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

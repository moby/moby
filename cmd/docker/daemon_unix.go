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
	// Special case for handling `docker help daemon`. When pkg/mflag is removed
	// we can support this on the daemon side, but that is not possible with
	// pkg/mflag because it uses os.Exit(1) instead of returning an error on
	// unexpected args.
	if len(args) == 0 || args[0] != "--help" {
		// Use os.Args[1:] so that "global" args are passed to dockerd
		args = stripDaemonArg(os.Args[1:])
	}

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

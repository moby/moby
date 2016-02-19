package main

import (
	"os"
	"os/exec"
	"syscall"
)

const daemonBinary = "dockerd"

// DaemonProxy acts as a cli.Handler to proxy calls to the daemon binary
type DaemonProxy struct{}

// NewDaemonProxy returns a new handler
func NewDaemonProxy() DaemonProxy {
	return DaemonProxy{}
}

// CmdDaemon execs dockerd with the same flags
// TODO: add a deprecation warning?
func (p DaemonProxy) CmdDaemon(args ...string) error {
	args = stripDaemonArg(os.Args[1:])

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

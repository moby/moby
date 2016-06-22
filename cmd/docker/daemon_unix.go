// +build daemon

package main

import (
	"fmt"

	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
)

const daemonBinary = "dockerd"

func newDaemonCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "daemon",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemon()
		},
	}
	cmd.SetHelpFunc(helpFunc)
	return cmd
}

// CmdDaemon execs dockerd with the same flags
func runDaemon() error {
	// Use os.Args[1:] so that "global" args are passed to dockerd
	return execDaemon(stripDaemonArg(os.Args[1:]))
}

func execDaemon(args []string) error {
	binaryPath, err := findDaemonBinary()
	if err != nil {
		return err
	}

	return syscall.Exec(
		binaryPath,
		append([]string{daemonBinary}, args...),
		os.Environ())
}

func helpFunc(cmd *cobra.Command, args []string) {
	if err := execDaemon([]string{"--help"}); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
	}
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

package ploop

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func ploopRunCmd(stdout io.Writer, args ...string) error {
	if verbosity != unsetVerbosity {
		if !strings.HasPrefix(args[0], "-v") {
			args = append(verbosityOpt, args...)
		}
		if verbosity > NoStdout && stdout == nil {
			stdout = os.Stdout
		}
	}
	var stderr bytes.Buffer
	cmd := exec.Command("ploop", args...)
	cmd.Stdout = stdout
	cmd.Stderr = &stderr

	if verbosity >= ShowCommands {
		fmt.Printf("Run: %s\n", strings.Join([]string{cmd.Path, strings.Join(cmd.Args[1:], " ")}, " "))
	}

	err := cmd.Run()
	if err == nil {
		return nil
	}

	// Command returned an error, get the stderr
	errStr := stderr.String()
	// Get the exit code (Unix-specific)
	if exiterr, ok := err.(*exec.ExitError); ok {
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			errCode := status.ExitStatus()
			return &Err{c: errCode, s: errStr}
		}
	}
	// unknown exit code
	return &Err{c: -1, s: errStr}
}

func ploop(args ...string) error {
	return ploopRunCmd(nil, args...)
}

func ploopOut(args ...string) (string, error) {
	var stdout bytes.Buffer
	// Output is reqired, make sure verbosity is not negative
	if verbosity < 0 {
		v := []string{"-v0"}
		args = append(v, args...)
	}
	ret := ploopRunCmd(&stdout, args...)
	out := stdout.String()
	// if verbosity requires so, print command's stdout
	if verbosity > NoStdout {
		fmt.Printf("%s", out)
	}
	return out, ret
}

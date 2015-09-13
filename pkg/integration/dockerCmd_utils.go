package integration

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/go-check/check"
)

var execCommand = exec.Command

// DockerCmdWithError executes a docker command that is supposed to fail and returns
// the output, the exit code and the error.
func DockerCmdWithError(dockerBinary string, args ...string) (string, int, error) {
	return RunCommandWithOutput(execCommand(dockerBinary, args...))
}

// DockerCmdWithStdoutStderr executes a docker command and returns the content of the
// stdout, stderr and the exit code. If a check.C is passed, it will fail and stop tests
// if the error is not nil.
func DockerCmdWithStdoutStderr(dockerBinary string, c *check.C, args ...string) (string, string, int) {
	stdout, stderr, status, err := RunCommandWithStdoutStderr(execCommand(dockerBinary, args...))
	if c != nil {
		c.Assert(err, check.IsNil, check.Commentf("%q failed with errors: %s, %v", strings.Join(args, " "), stderr, err))
	}
	return stdout, stderr, status
}

// DockerCmd executes a docker command and returns the output and the exit code. If the
// command returns an error, it will fail and stop the tests.
func DockerCmd(dockerBinary string, c *check.C, args ...string) (string, int) {
	out, status, err := RunCommandWithOutput(execCommand(dockerBinary, args...))
	c.Assert(err, check.IsNil, check.Commentf("%q failed with errors: %s, %v", strings.Join(args, " "), out, err))
	return out, status
}

// DockerCmdWithTimeout executes a docker command with a timeout, and returns the output,
// the exit code and the error (if any).
func DockerCmdWithTimeout(dockerBinary string, timeout time.Duration, args ...string) (string, int, error) {
	out, status, err := RunCommandWithOutputAndTimeout(execCommand(dockerBinary, args...), timeout)
	if err != nil {
		return out, status, fmt.Errorf("%q failed with errors: %v : %q", strings.Join(args, " "), err, out)
	}
	return out, status, err
}

// DockerCmdInDir executes a docker command in a directory and returns the output, the
// exit code and the error (if any).
func DockerCmdInDir(dockerBinary string, path string, args ...string) (string, int, error) {
	dockerCommand := execCommand(dockerBinary, args...)
	dockerCommand.Dir = path
	out, status, err := RunCommandWithOutput(dockerCommand)
	if err != nil {
		return out, status, fmt.Errorf("%q failed with errors: %v : %q", strings.Join(args, " "), err, out)
	}
	return out, status, err
}

// DockerCmdInDirWithTimeout executes a docker command in a directory with a timeout and
// returns the output, the exit code and the error (if any).
func DockerCmdInDirWithTimeout(dockerBinary string, timeout time.Duration, path string, args ...string) (string, int, error) {
	dockerCommand := execCommand(dockerBinary, args...)
	dockerCommand.Dir = path
	out, status, err := RunCommandWithOutputAndTimeout(dockerCommand, timeout)
	if err != nil {
		return out, status, fmt.Errorf("%q failed with errors: %v : %q", strings.Join(args, " "), err, out)
	}
	return out, status, err
}

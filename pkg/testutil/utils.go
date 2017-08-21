package testutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/docker/pkg/system"
)

func runCommandWithOutput(cmd *exec.Cmd) (output string, exitCode int, err error) {
	out, err := cmd.CombinedOutput()
	exitCode = system.ProcessExitCode(err)
	output = string(out)
	return
}

// RunCommandPipelineWithOutput runs the array of commands with the output
// of each pipelined with the following (like cmd1 | cmd2 | cmd3 would do).
// It returns the final output, the exitCode different from 0 and the error
// if something bad happened.
func RunCommandPipelineWithOutput(cmds ...*exec.Cmd) (output string, exitCode int, err error) {
	if len(cmds) < 2 {
		return "", 0, errors.New("pipeline does not have multiple cmds")
	}

	// connect stdin of each cmd to stdout pipe of previous cmd
	for i, cmd := range cmds {
		if i > 0 {
			prevCmd := cmds[i-1]
			cmd.Stdin, err = prevCmd.StdoutPipe()

			if err != nil {
				return "", 0, fmt.Errorf("cannot set stdout pipe for %s: %v", cmd.Path, err)
			}
		}
	}

	// start all cmds except the last
	for _, cmd := range cmds[:len(cmds)-1] {
		if err = cmd.Start(); err != nil {
			return "", 0, fmt.Errorf("starting %s failed with error: %v", cmd.Path, err)
		}
	}

	defer func() {
		var pipeErrMsgs []string
		// wait all cmds except the last to release their resources
		for _, cmd := range cmds[:len(cmds)-1] {
			if pipeErr := cmd.Wait(); pipeErr != nil {
				pipeErrMsgs = append(pipeErrMsgs, fmt.Sprintf("command %s failed with error: %v", cmd.Path, pipeErr))
			}
		}
		if len(pipeErrMsgs) > 0 && err == nil {
			err = fmt.Errorf("pipelineError from Wait: %v", strings.Join(pipeErrMsgs, ", "))
		}
	}()

	// wait on last cmd
	return runCommandWithOutput(cmds[len(cmds)-1])
}

// RandomTmpDirPath provides a temporary path with rand string appended.
// does not create or checks if it exists.
func RandomTmpDirPath(s string, platform string) string {
	tmp := "/tmp"
	if platform == "windows" {
		tmp = os.Getenv("TEMP")
	}
	path := filepath.Join(tmp, fmt.Sprintf("%s.%s", s, stringutils.GenerateRandomAlphaOnlyString(10)))
	if platform == "windows" {
		return filepath.FromSlash(path) // Using \
	}
	return filepath.ToSlash(path) // Using /
}

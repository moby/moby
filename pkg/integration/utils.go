package integration

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/pkg/stringutils"
)

// GetExitCode returns the ExitStatus of the specified error if its type is
// exec.ExitError, returns 0 and an error otherwise.
func GetExitCode(err error) (int, error) {
	exitCode := 0
	if exiterr, ok := err.(*exec.ExitError); ok {
		if procExit, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			return procExit.ExitStatus(), nil
		}
	}
	return exitCode, fmt.Errorf("failed to get exit code")
}

// ProcessExitCode process the specified error and returns the exit status code
// if the error was of type exec.ExitError, returns nothing otherwise.
func ProcessExitCode(err error) (exitCode int) {
	if err != nil {
		var exiterr error
		if exitCode, exiterr = GetExitCode(err); exiterr != nil {
			// TODO: Fix this so we check the error's text.
			// we've failed to retrieve exit code, so we set it to 127
			exitCode = 127
		}
	}
	return
}

// IsKilled process the specified error and returns whether the process was killed or not.
func IsKilled(err error) bool {
	if exitErr, ok := err.(*exec.ExitError); ok {
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		if !ok {
			return false
		}
		// status.ExitStatus() is required on Windows because it does not
		// implement Signal() nor Signaled(). Just check it had a bad exit
		// status could mean it was killed (and in tests we do kill)
		return (status.Signaled() && status.Signal() == os.Kill) || status.ExitStatus() != 0
	}
	return false
}

// RunCommandWithOutput runs the specified command and returns the combined output (stdout/stderr)
// with the exitCode different from 0 and the error if something bad happened
func RunCommandWithOutput(cmd *exec.Cmd) (output string, exitCode int, err error) {
	exitCode = 0
	out, err := cmd.CombinedOutput()
	exitCode = ProcessExitCode(err)
	output = string(out)
	return
}

// RunCommandWithStdoutStderr runs the specified command and returns stdout and stderr separately
// with the exitCode different from 0 and the error if something bad happened
func RunCommandWithStdoutStderr(cmd *exec.Cmd) (stdout string, stderr string, exitCode int, err error) {
	var (
		stderrBuffer, stdoutBuffer bytes.Buffer
	)
	exitCode = 0
	cmd.Stderr = &stderrBuffer
	cmd.Stdout = &stdoutBuffer
	err = cmd.Run()
	exitCode = ProcessExitCode(err)

	stdout = stdoutBuffer.String()
	stderr = stderrBuffer.String()
	return
}

// RunCommandWithOutputForDuration runs the specified command "timeboxed" by the specified duration.
// If the process is still running when the timebox is finished, the process will be killed and .
// It will returns the output with the exitCode different from 0 and the error if something bad happened
// and a boolean whether it has been killed or not.
func RunCommandWithOutputForDuration(cmd *exec.Cmd, duration time.Duration) (output string, exitCode int, timedOut bool, err error) {
	var outputBuffer bytes.Buffer
	if cmd.Stdout != nil {
		err = errors.New("cmd.Stdout already set")
		return
	}
	cmd.Stdout = &outputBuffer

	if cmd.Stderr != nil {
		err = errors.New("cmd.Stderr already set")
		return
	}
	cmd.Stderr = &outputBuffer

	// Start the command in the main thread..
	err = cmd.Start()
	if err != nil {
		err = fmt.Errorf("Fail to start command %v : %v", cmd, err)
	}

	type exitInfo struct {
		exitErr  error
		exitCode int
	}

	done := make(chan exitInfo, 1)

	go func() {
		// And wait for it to exit in the goroutine :)
		info := exitInfo{}
		info.exitErr = cmd.Wait()
		info.exitCode = ProcessExitCode(info.exitErr)
		done <- info
	}()

	select {
	case <-time.After(duration):
		killErr := cmd.Process.Kill()
		if killErr != nil {
			fmt.Printf("failed to kill (pid=%d): %v\n", cmd.Process.Pid, killErr)
		}
		timedOut = true
	case info := <-done:
		err = info.exitErr
		exitCode = info.exitCode
	}
	output = outputBuffer.String()
	return
}

var errCmdTimeout = fmt.Errorf("command timed out")

// RunCommandWithOutputAndTimeout runs the specified command "timeboxed" by the specified duration.
// It returns the output with the exitCode different from 0 and the error if something bad happened or
// if the process timed out (and has been killed).
func RunCommandWithOutputAndTimeout(cmd *exec.Cmd, timeout time.Duration) (output string, exitCode int, err error) {
	var timedOut bool
	output, exitCode, timedOut, err = RunCommandWithOutputForDuration(cmd, timeout)
	if timedOut {
		err = errCmdTimeout
	}
	return
}

// RunCommand runs the specified command and returns the exitCode different from 0
// and the error if something bad happened.
func RunCommand(cmd *exec.Cmd) (exitCode int, err error) {
	exitCode = 0
	err = cmd.Run()
	exitCode = ProcessExitCode(err)
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

	var pipelineError error
	defer func() {
		// wait all cmds except the last to release their resources
		for _, cmd := range cmds[:len(cmds)-1] {
			if err := cmd.Wait(); err != nil {
				pipelineError = fmt.Errorf("command %s failed with error: %v", cmd.Path, err)
				break
			}
		}
	}()
	if pipelineError != nil {
		return "", 0, pipelineError
	}

	// wait on last cmd
	return RunCommandWithOutput(cmds[len(cmds)-1])
}

// UnmarshalJSON deserialize a JSON in the given interface.
func UnmarshalJSON(data []byte, result interface{}) error {
	if err := json.Unmarshal(data, result); err != nil {
		return err
	}

	return nil
}

// ConvertSliceOfStringsToMap converts a slices of string in a map
// with the strings as key and an empty string as values.
func ConvertSliceOfStringsToMap(input []string) map[string]struct{} {
	output := make(map[string]struct{})
	for _, v := range input {
		output[v] = struct{}{}
	}
	return output
}

// CompareDirectoryEntries compares two sets of FileInfo (usually taken from a directory)
// and returns an error if different.
func CompareDirectoryEntries(e1 []os.FileInfo, e2 []os.FileInfo) error {
	var (
		e1Entries = make(map[string]struct{})
		e2Entries = make(map[string]struct{})
	)
	for _, e := range e1 {
		e1Entries[e.Name()] = struct{}{}
	}
	for _, e := range e2 {
		e2Entries[e.Name()] = struct{}{}
	}
	if !reflect.DeepEqual(e1Entries, e2Entries) {
		return fmt.Errorf("entries differ")
	}
	return nil
}

// ListTar lists the entries of a tar.
func ListTar(f io.Reader) ([]string, error) {
	tr := tar.NewReader(f)
	var entries []string

	for {
		th, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			return entries, nil
		}
		if err != nil {
			return entries, err
		}
		entries = append(entries, th.Name)
	}
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

// ConsumeWithSpeed reads chunkSize bytes from reader before sleeping
// for interval duration. Returns total read bytes. Send true to the
// stop channel to return before reading to EOF on the reader.
func ConsumeWithSpeed(reader io.Reader, chunkSize int, interval time.Duration, stop chan bool) (n int, err error) {
	buffer := make([]byte, chunkSize)
	for {
		var readBytes int
		readBytes, err = reader.Read(buffer)
		n += readBytes
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
		select {
		case <-stop:
			return
		case <-time.After(interval):
		}
	}
}

// ParseCgroupPaths parses 'procCgroupData', which is output of '/proc/<pid>/cgroup', and returns
// a map which cgroup name as key and path as value.
func ParseCgroupPaths(procCgroupData string) map[string]string {
	cgroupPaths := map[string]string{}
	for _, line := range strings.Split(procCgroupData, "\n") {
		parts := strings.Split(line, ":")
		if len(parts) != 3 {
			continue
		}
		cgroupPaths[parts[1]] = parts[2]
	}
	return cgroupPaths
}

// ChannelBuffer holds a chan of byte array that can be populate in a goroutine.
type ChannelBuffer struct {
	C chan []byte
}

// Write implements Writer.
func (c *ChannelBuffer) Write(b []byte) (int, error) {
	c.C <- b
	return len(b), nil
}

// Close closes the go channel.
func (c *ChannelBuffer) Close() error {
	close(c.C)
	return nil
}

// ReadTimeout reads the content of the channel in the specified byte array with
// the specified duration as timeout.
func (c *ChannelBuffer) ReadTimeout(p []byte, n time.Duration) (int, error) {
	select {
	case b := <-c.C:
		return copy(p[0:], b), nil
	case <-time.After(n):
		return -1, fmt.Errorf("timeout reading from channel")
	}
}

// RunAtDifferentDate runs the specified function with the given time.
// It changes the date of the system, which can led to weird behaviors.
func RunAtDifferentDate(date time.Time, block func()) {
	// Layout for date. MMDDhhmmYYYY
	const timeLayout = "010203042006"
	// Ensure we bring time back to now
	now := time.Now().Format(timeLayout)
	dateReset := exec.Command("date", now)
	defer RunCommand(dateReset)

	dateChange := exec.Command("date", date.Format(timeLayout))
	RunCommand(dateChange)
	block()
	return
}

// +build windows

package client

import (
	"fmt"
	"io"

	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
)

// Process is the structure pertaining to a process running in a utility VM.
type process struct {
	Process hcsshim.Process
	Stdin   io.WriteCloser
	Stdout  io.ReadCloser
	Stderr  io.ReadCloser
}

// createUtilsProcess is a convenient wrapper for hcsshim.createUtilsProcess to use when
// communicating with a utility VM.
func (config *Config) createUtilsProcess(commandLine string) (process, error) {
	logrus.Debugf("opengcs: createUtilsProcess")

	if config.Uvm == nil {
		return process{}, fmt.Errorf("cannot create utils process as no utility VM is in configuration")
	}

	var (
		err  error
		proc process
	)

	env := make(map[string]string)
	env["PATH"] = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:"
	processConfig := &hcsshim.ProcessConfig{
		EmulateConsole:    false,
		CreateStdInPipe:   true,
		CreateStdOutPipe:  true,
		CreateStdErrPipe:  true,
		CreateInUtilityVm: true,
		WorkingDirectory:  "/bin",
		Environment:       env,
		CommandLine:       commandLine,
	}
	proc.Process, err = config.Uvm.CreateProcess(processConfig)
	if err != nil {
		return process{}, fmt.Errorf("failed to create process (%+v) in utility VM: %s", config, err)
	}

	if proc.Stdin, proc.Stdout, proc.Stderr, err = proc.Process.Stdio(); err != nil {
		proc.Process.Kill() // Should this have a timeout?
		proc.Process.Close()
		return process{}, fmt.Errorf("failed to get stdio pipes for process %+v: %s", config, err)
	}

	logrus.Debugf("opengcs: createUtilsProcess success: pid %d", proc.Process.Pid())
	return proc, nil
}

// RunProcess runs the given command line program in the utilityVM. It takes in
// an input to the reader to feed into stdin and returns stdout to output.
// IMPORTANT: It is the responsibility of the caller to call Close() on the returned process.
func (config *Config) RunProcess(commandLine string, stdin io.Reader, stdout io.Writer, stderr io.Writer) (hcsshim.Process, error) {
	logrus.Debugf("opengcs: RunProcess: %s", commandLine)
	process, err := config.createUtilsProcess(commandLine)
	if err != nil {
		return nil, err
	}

	// Send the data into the process's stdin
	if stdin != nil {
		if _, err = copyWithTimeout(process.Stdin,
			stdin,
			0,
			config.UvmTimeoutSeconds,
			fmt.Sprintf("send to stdin of %s", commandLine)); err != nil {
			return nil, err
		}

		// Don't need stdin now we've sent everything. This signals GCS that we are finished sending data.
		if err := process.Process.CloseStdin(); err != nil {
			return nil, err
		}
	}

	if stdout != nil {
		// Copy the data over to the writer.
		if _, err := copyWithTimeout(stdout,
			process.Stdout,
			0,
			config.UvmTimeoutSeconds,
			fmt.Sprintf("RunProcess: copy back from %s", commandLine)); err != nil {
			return nil, err
		}
	}

	if stderr != nil {
		// Copy the data over to the writer.
		if _, err := copyWithTimeout(stderr,
			process.Stderr,
			0,
			config.UvmTimeoutSeconds,
			fmt.Sprintf("RunProcess: copy back from %s", commandLine)); err != nil {
			return nil, err
		}
	}

	logrus.Debugf("opengcs: runProcess success: %s", commandLine)
	return process.Process, nil
}

// +build windows

package client

import (
	"fmt"
	"io"

	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
)

// Process is the structure pertaining to a process running in a utility VM.
type process struct {
	Process hcsshim.Process
	Stdin   io.WriteCloser
	Stdout  io.ReadCloser
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
		return process{}, fmt.Errorf("opengcs: createUtilsProcess: CreateProcess %+v failed %s", config, err)
	}

	if proc.Stdin, proc.Stdout, _, err = proc.Process.Stdio(); err != nil {
		proc.Process.Kill() // Should this have a timeout?
		proc.Process.Close()
		return process{}, fmt.Errorf("opengcs: createUtilsProcess: failed to get Stdio pipes %s", err)
	}

	logrus.Debugf("opengcs: createUtilsProcess success: pid %d", proc.Process.Pid())
	return proc, nil
}

// RunProcess runs the given command line program in the utilityVM. It takes in
// an input to the reader to feed into stdin and returns stdout to output.
func (config *Config) RunProcess(commandLine string, input io.Reader, output io.Writer) error {
	logrus.Debugf("opengcs: RunProcess: %s", commandLine)
	process, err := config.createUtilsProcess(commandLine)
	if err != nil {
		return err
	}
	defer process.Process.Close()

	// Send the data into the process's stdin
	if input != nil {
		if _, err = copyWithTimeout(process.Stdin,
			input,
			0,
			config.UvmTimeoutSeconds,
			fmt.Sprintf("send to stdin of %s", commandLine)); err != nil {
			return err
		}

		// Don't need stdin now we've sent everything. This signals GCS that we are finished sending data.
		if err := process.Process.CloseStdin(); err != nil {
			return err
		}
	}

	if output != nil {
		// Copy the data over to the writer.
		if _, err := copyWithTimeout(output,
			process.Stdout,
			0,
			config.UvmTimeoutSeconds,
			fmt.Sprintf("RunProcess: copy back from %s", commandLine)); err != nil {
			return err
		}
	}

	logrus.Debugf("opengcs: runProcess success: %s", commandLine)
	return nil
}

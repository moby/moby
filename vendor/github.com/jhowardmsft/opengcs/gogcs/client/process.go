// +build windows

package client

// TODO @jhowardmsft - This will move to Microsoft/opengcs soon

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

// +build windows

package opengcs

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
func createUtilsProcess(uvm hcsshim.Container) (process, error) {
	logrus.Debugf("opengcs: createUtilsProcess")

	if uvm == nil {
		return process{}, fmt.Errorf("opengcs: createUtilsProcess: No utility VM supplied")
	}

	var (
		err  error
		proc process
	)

	env := make(map[string]string)
	env["PATH"] = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/root/integration"
	config := &hcsshim.ProcessConfig{
		EmulateConsole:    false,
		CreateStdInPipe:   true,
		CreateStdOutPipe:  true,
		CreateStdErrPipe:  true,
		CreateInUtilityVm: true,
		WorkingDirectory:  "/bin",
		Environment:       env,
		CommandLine:       "./svm_utils",
	}
	proc.Process, err = uvm.CreateProcess(config)
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

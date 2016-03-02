package hcsshim

import (
	"encoding/json"
	"io"
	"syscall"

	"github.com/Microsoft/go-winio"
	"github.com/Sirupsen/logrus"
)

// CreateProcessParams is used as both the input of CreateProcessInComputeSystem
// and to convert the parameters to JSON for passing onto the HCS
type CreateProcessParams struct {
	ApplicationName  string
	CommandLine      string
	WorkingDirectory string
	Environment      map[string]string
	EmulateConsole   bool
	ConsoleSize      [2]int
}

// makeOpenFiles calls winio.MakeOpenFile for each handle in a slice but closes all the handles
// if there is an error.
func makeOpenFiles(hs []syscall.Handle) (_ []io.ReadWriteCloser, err error) {
	fs := make([]io.ReadWriteCloser, len(hs))
	for i, h := range hs {
		if h != syscall.Handle(0) {
			if err == nil {
				fs[i], err = winio.MakeOpenFile(h)
			}
			if err != nil {
				syscall.Close(h)
			}
		}
	}
	if err != nil {
		for _, f := range fs {
			if f != nil {
				f.Close()
			}
		}
		return nil, err
	}
	return fs, nil
}

// CreateProcessInComputeSystem starts a process in a container. This is invoked, for example,
// as a result of docker run, docker exec, or RUN in Dockerfile. If successful,
// it returns the PID of the process.
func CreateProcessInComputeSystem(id string, useStdin bool, useStdout bool, useStderr bool, params CreateProcessParams) (_ uint32, _ io.WriteCloser, _ io.ReadCloser, _ io.ReadCloser, err error) {
	title := "HCSShim::CreateProcessInComputeSystem"
	logrus.Debugf(title+" id=%s", id)

	// If we are not emulating a console, ignore any console size passed to us
	if !params.EmulateConsole {
		params.ConsoleSize[0] = 0
		params.ConsoleSize[1] = 0
	}

	paramsJson, err := json.Marshal(params)
	if err != nil {
		return
	}

	logrus.Debugf(title+" - Calling Win32 %s %s", id, paramsJson)

	var pid uint32

	handles := make([]syscall.Handle, 3)
	var stdinParam, stdoutParam, stderrParam *syscall.Handle
	if useStdin {
		stdinParam = &handles[0]
	}
	if useStdout {
		stdoutParam = &handles[1]
	}
	if useStderr {
		stderrParam = &handles[2]
	}

	err = createProcessWithStdHandlesInComputeSystem(id, string(paramsJson), &pid, stdinParam, stdoutParam, stderrParam)
	if err != nil {
		herr := makeErrorf(err, title, "id=%s params=%v", id, params)
		// Windows TP4: Hyper-V Containers may return this error with more than one
		// concurrent exec. Do not log it as an error
		if err != WSAEINVAL {
			logrus.Error(herr)
		}
		err = herr
		return
	}

	pipes, err := makeOpenFiles(handles)
	if err != nil {
		return
	}

	logrus.Debugf(title+" - succeeded id=%s params=%s pid=%d", id, paramsJson, pid)
	return pid, pipes[0], pipes[1], pipes[2], nil
}

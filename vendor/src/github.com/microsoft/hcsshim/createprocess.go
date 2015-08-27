package hcsshim

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"syscall"
	"unsafe"

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

// pipe struct used for the stdin/stdout/stderr pipes
type pipe struct {
	handle syscall.Handle
}

func makePipe(h syscall.Handle) *pipe {
	p := &pipe{h}
	runtime.SetFinalizer(p, (*pipe).closeHandle)
	return p
}

func (p *pipe) closeHandle() {
	if p.handle != 0 {
		syscall.CloseHandle(p.handle)
		p.handle = 0
	}
}

func (p *pipe) Close() error {
	p.closeHandle()
	runtime.SetFinalizer(p, nil)
	return nil
}

func (p *pipe) Read(b []byte) (int, error) {
	// syscall.Read returns 0, nil on ERROR_BROKEN_PIPE, but for
	// our purposes this should indicate EOF. This may be a go bug.
	var read uint32
	err := syscall.ReadFile(p.handle, b, &read, nil)
	if err != nil {
		if err == syscall.ERROR_BROKEN_PIPE {
			return 0, io.EOF
		}
		return 0, err
	}
	return int(read), nil
}

func (p *pipe) Write(b []byte) (int, error) {
	return syscall.Write(p.handle, b)
}

// CreateProcessInComputeSystem starts a process in a container. This is invoked, for example,
// as a result of docker run, docker exec, or RUN in Dockerfile. If successful,
// it returns the PID of the process.
func CreateProcessInComputeSystem(id string, useStdin bool, useStdout bool, useStderr bool, params CreateProcessParams) (processid uint32, stdin io.WriteCloser, stdout io.ReadCloser, stderr io.ReadCloser, err error) {

	title := "HCSShim::CreateProcessInComputeSystem"
	logrus.Debugf(title+"id=%s params=%s", id, params)

	// Load the DLL and get a handle to the procedure we need
	dll, proc, err := loadAndFind(procCreateProcessWithStdHandlesInComputeSystem)
	if dll != nil {
		defer dll.Release()
	}
	if err != nil {
		return
	}

	// Convert id to uint16 pointer for calling the procedure
	idp, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion of id %s to pointer %s", id, err)
		logrus.Error(err)
		return
	}

	// If we are not emulating a console, ignore any console size passed to us
	if !params.EmulateConsole {
		params.ConsoleSize[0] = 0
		params.ConsoleSize[1] = 0
	}

	paramsJson, err := json.Marshal(params)
	if err != nil {
		err = fmt.Errorf(title+" - Failed to marshall params %v %s", params, err)
		return
	}

	// Convert paramsJson to uint16 pointer for calling the procedure
	paramsJsonp, err := syscall.UTF16PtrFromString(string(paramsJson))
	if err != nil {
		return
	}

	// Get a POINTER to variable to take the pid outparm
	pid := new(uint32)

	logrus.Debugf(title+" - Calling the procedure itself %s %s", id, paramsJson)

	var stdinHandle, stdoutHandle, stderrHandle syscall.Handle
	var stdinParam, stdoutParam, stderrParam uintptr
	if useStdin {
		stdinParam = uintptr(unsafe.Pointer(&stdinHandle))
	}
	if useStdout {
		stdoutParam = uintptr(unsafe.Pointer(&stdoutHandle))
	}
	if useStderr {
		stderrParam = uintptr(unsafe.Pointer(&stderrHandle))
	}

	// Call the procedure itself.
	r1, _, _ := proc.Call(
		uintptr(unsafe.Pointer(idp)),
		uintptr(unsafe.Pointer(paramsJsonp)),
		uintptr(unsafe.Pointer(pid)),
		stdinParam,
		stdoutParam,
		stderrParam)

	use(unsafe.Pointer(idp))
	use(unsafe.Pointer(paramsJsonp))

	if r1 != 0 {
		err = fmt.Errorf(title+" - Win32 API call returned error r1=%d err=%s id=%s params=%v", r1, syscall.Errno(r1), id, params)
		logrus.Error(err)
		return
	}

	if useStdin {
		stdin = makePipe(stdinHandle)
	}
	if useStdout {
		stdout = makePipe(stdoutHandle)
	}
	if useStderr {
		stderr = makePipe(stderrHandle)
	}

	logrus.Debugf(title+" - succeeded id=%s params=%s pid=%d", id, paramsJson, *pid)
	return *pid, stdin, stdout, stderr, nil
}

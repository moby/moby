package hcsshim

import (
	"encoding/json"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Sirupsen/logrus"
)

// processParameters is use to both the input of CreateProcessInComputeSystem
// and to convert the parameters to JSON for passing onto the HCS
type CreateProcessParams struct {
	ApplicationName                   string
	CommandLine                       string
	WorkingDirectory                  string
	StdInPipe, StdOutPipe, StdErrPipe string
	Environment                       map[string]string
	EmulateConsole                    bool
	ConsoleSize                       [2]int
}

// CreateProcessInComputeSystem starts a process in a container. This is invoked, for example,
// as a result of docker run, docker exec, or RUN in Dockerfile. If successful,
// it returns the PID of the process.
func CreateProcessInComputeSystem(id string, params CreateProcessParams) (processid uint32, err error) {

	title := "HCSShim::CreateProcessInComputeSystem"
	logrus.Debugf(title+"id=%s params=%s", id, params)

	// Load the DLL and get a handle to the procedure we need
	dll, proc, err := loadAndFind(procCreateProcessInComputeSystem)
	if dll != nil {
		defer dll.Release()
	}
	if err != nil {
		return 0, err
	}

	// Convert id to uint16 pointer for calling the procedure
	idp, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		err = fmt.Errorf(title+" - Failed conversion of id %s to pointer %s", id, err)
		logrus.Error(err)
		return 0, err
	}

	// If we are not emulating a console, ignore any console size passed to us
	if !params.EmulateConsole {
		params.ConsoleSize[0] = 0
		params.ConsoleSize[1] = 0
	}

	paramsJson, err := json.Marshal(params)
	if err != nil {
		err = fmt.Errorf(title+" - Failed to marshall params %s %s", params, err)
		return 0, err
	}

	// Convert paramsJson to uint16 pointer for calling the procedure
	paramsJsonp, err := syscall.UTF16PtrFromString(string(paramsJson))
	if err != nil {
		return 0, err
	}

	// Get a POINTER to variable to take the pid outparm
	pid := new(uint32)

	logrus.Debugf(title+" - Calling the procedure itself %s %s", id, paramsJson)

	// Call the procedure itself.
	r1, _, _ := proc.Call(
		uintptr(unsafe.Pointer(idp)),
		uintptr(unsafe.Pointer(paramsJsonp)),
		uintptr(unsafe.Pointer(pid)))

	use(unsafe.Pointer(idp))
	use(unsafe.Pointer(paramsJsonp))

	if r1 != 0 {
		err = fmt.Errorf(title+" - Win32 API call returned error r1=%d err=%s id=%s params=%s", r1, syscall.Errno(r1), id, params)
		logrus.Error(err)
		return 0, err
	}

	logrus.Debugf(title+" - succeeded id=%s params=%s pid=%d", id, paramsJson, *pid)
	return *pid, nil
}

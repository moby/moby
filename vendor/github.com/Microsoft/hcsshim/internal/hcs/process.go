package hcs

import (
	"encoding/json"
	"io"
	"sync"
	"syscall"
	"time"

	"github.com/Microsoft/hcsshim/internal/interop"
	"github.com/sirupsen/logrus"
)

// ContainerError is an error encountered in HCS
type Process struct {
	handleLock     sync.RWMutex
	handle         hcsProcess
	processID      int
	system         *System
	cachedPipes    *cachedPipes
	callbackNumber uintptr
}

type cachedPipes struct {
	stdIn  syscall.Handle
	stdOut syscall.Handle
	stdErr syscall.Handle
}

type processModifyRequest struct {
	Operation   string
	ConsoleSize *consoleSize `json:",omitempty"`
	CloseHandle *closeHandle `json:",omitempty"`
}

type consoleSize struct {
	Height uint16
	Width  uint16
}

type closeHandle struct {
	Handle string
}

type ProcessStatus struct {
	ProcessID      uint32
	Exited         bool
	ExitCode       uint32
	LastWaitResult int32
}

const (
	stdIn  string = "StdIn"
	stdOut string = "StdOut"
	stdErr string = "StdErr"
)

const (
	modifyConsoleSize string = "ConsoleSize"
	modifyCloseHandle string = "CloseHandle"
)

// Pid returns the process ID of the process within the container.
func (process *Process) Pid() int {
	return process.processID
}

// SystemID returns the ID of the process's compute system.
func (process *Process) SystemID() string {
	return process.system.ID()
}

// Kill signals the process to terminate but does not wait for it to finish terminating.
func (process *Process) Kill() error {
	process.handleLock.RLock()
	defer process.handleLock.RUnlock()
	operation := "Kill"
	title := "hcsshim::Process::" + operation
	logrus.Debugf(title+" processid=%d", process.processID)

	if process.handle == 0 {
		return makeProcessError(process, operation, ErrAlreadyClosed, nil)
	}

	var resultp *uint16
	err := hcsTerminateProcess(process.handle, &resultp)
	events := processHcsResult(resultp)
	if err != nil {
		return makeProcessError(process, operation, err, events)
	}

	logrus.Debugf(title+" succeeded processid=%d", process.processID)
	return nil
}

// Wait waits for the process to exit.
func (process *Process) Wait() error {
	operation := "Wait"
	title := "hcsshim::Process::" + operation
	logrus.Debugf(title+" processid=%d", process.processID)

	err := waitForNotification(process.callbackNumber, hcsNotificationProcessExited, nil)
	if err != nil {
		return makeProcessError(process, operation, err, nil)
	}

	logrus.Debugf(title+" succeeded processid=%d", process.processID)
	return nil
}

// WaitTimeout waits for the process to exit or the duration to elapse. It returns
// false if timeout occurs.
func (process *Process) WaitTimeout(timeout time.Duration) error {
	operation := "WaitTimeout"
	title := "hcsshim::Process::" + operation
	logrus.Debugf(title+" processid=%d", process.processID)

	err := waitForNotification(process.callbackNumber, hcsNotificationProcessExited, &timeout)
	if err != nil {
		return makeProcessError(process, operation, err, nil)
	}

	logrus.Debugf(title+" succeeded processid=%d", process.processID)
	return nil
}

// ResizeConsole resizes the console of the process.
func (process *Process) ResizeConsole(width, height uint16) error {
	process.handleLock.RLock()
	defer process.handleLock.RUnlock()
	operation := "ResizeConsole"
	title := "hcsshim::Process::" + operation
	logrus.Debugf(title+" processid=%d", process.processID)

	if process.handle == 0 {
		return makeProcessError(process, operation, ErrAlreadyClosed, nil)
	}

	modifyRequest := processModifyRequest{
		Operation: modifyConsoleSize,
		ConsoleSize: &consoleSize{
			Height: height,
			Width:  width,
		},
	}

	modifyRequestb, err := json.Marshal(modifyRequest)
	if err != nil {
		return err
	}

	modifyRequestStr := string(modifyRequestb)

	var resultp *uint16
	err = hcsModifyProcess(process.handle, modifyRequestStr, &resultp)
	events := processHcsResult(resultp)
	if err != nil {
		return makeProcessError(process, operation, err, events)
	}

	logrus.Debugf(title+" succeeded processid=%d", process.processID)
	return nil
}

func (process *Process) Properties() (*ProcessStatus, error) {
	process.handleLock.RLock()
	defer process.handleLock.RUnlock()
	operation := "Properties"
	title := "hcsshim::Process::" + operation
	logrus.Debugf(title+" processid=%d", process.processID)

	if process.handle == 0 {
		return nil, makeProcessError(process, operation, ErrAlreadyClosed, nil)
	}

	var (
		resultp     *uint16
		propertiesp *uint16
	)
	err := hcsGetProcessProperties(process.handle, &propertiesp, &resultp)
	events := processHcsResult(resultp)
	if err != nil {
		return nil, makeProcessError(process, operation, err, events)
	}

	if propertiesp == nil {
		return nil, ErrUnexpectedValue
	}
	propertiesRaw := interop.ConvertAndFreeCoTaskMemBytes(propertiesp)

	properties := &ProcessStatus{}
	if err := json.Unmarshal(propertiesRaw, properties); err != nil {
		return nil, makeProcessError(process, operation, err, nil)
	}

	logrus.Debugf(title+" succeeded processid=%d, properties=%s", process.processID, propertiesRaw)
	return properties, nil
}

// ExitCode returns the exit code of the process. The process must have
// already terminated.
func (process *Process) ExitCode() (int, error) {
	operation := "ExitCode"
	properties, err := process.Properties()
	if err != nil {
		return 0, makeProcessError(process, operation, err, nil)
	}

	if properties.Exited == false {
		return 0, makeProcessError(process, operation, ErrInvalidProcessState, nil)
	}

	if properties.LastWaitResult != 0 {
		return 0, makeProcessError(process, operation, syscall.Errno(properties.LastWaitResult), nil)
	}

	return int(properties.ExitCode), nil
}

// Stdio returns the stdin, stdout, and stderr pipes, respectively. Closing
// these pipes does not close the underlying pipes; it should be possible to
// call this multiple times to get multiple interfaces.
func (process *Process) Stdio() (io.WriteCloser, io.ReadCloser, io.ReadCloser, error) {
	process.handleLock.RLock()
	defer process.handleLock.RUnlock()
	operation := "Stdio"
	title := "hcsshim::Process::" + operation
	logrus.Debugf(title+" processid=%d", process.processID)

	if process.handle == 0 {
		return nil, nil, nil, makeProcessError(process, operation, ErrAlreadyClosed, nil)
	}

	var stdIn, stdOut, stdErr syscall.Handle

	if process.cachedPipes == nil {
		var (
			processInfo hcsProcessInformation
			resultp     *uint16
		)
		err := hcsGetProcessInfo(process.handle, &processInfo, &resultp)
		events := processHcsResult(resultp)
		if err != nil {
			return nil, nil, nil, makeProcessError(process, operation, err, events)
		}

		stdIn, stdOut, stdErr = processInfo.StdInput, processInfo.StdOutput, processInfo.StdError
	} else {
		// Use cached pipes
		stdIn, stdOut, stdErr = process.cachedPipes.stdIn, process.cachedPipes.stdOut, process.cachedPipes.stdErr

		// Invalidate the cache
		process.cachedPipes = nil
	}

	pipes, err := makeOpenFiles([]syscall.Handle{stdIn, stdOut, stdErr})
	if err != nil {
		return nil, nil, nil, makeProcessError(process, operation, err, nil)
	}

	logrus.Debugf(title+" succeeded processid=%d", process.processID)
	return pipes[0], pipes[1], pipes[2], nil
}

// CloseStdin closes the write side of the stdin pipe so that the process is
// notified on the read side that there is no more data in stdin.
func (process *Process) CloseStdin() error {
	process.handleLock.RLock()
	defer process.handleLock.RUnlock()
	operation := "CloseStdin"
	title := "hcsshim::Process::" + operation
	logrus.Debugf(title+" processid=%d", process.processID)

	if process.handle == 0 {
		return makeProcessError(process, operation, ErrAlreadyClosed, nil)
	}

	modifyRequest := processModifyRequest{
		Operation: modifyCloseHandle,
		CloseHandle: &closeHandle{
			Handle: stdIn,
		},
	}

	modifyRequestb, err := json.Marshal(modifyRequest)
	if err != nil {
		return err
	}

	modifyRequestStr := string(modifyRequestb)

	var resultp *uint16
	err = hcsModifyProcess(process.handle, modifyRequestStr, &resultp)
	events := processHcsResult(resultp)
	if err != nil {
		return makeProcessError(process, operation, err, events)
	}

	logrus.Debugf(title+" succeeded processid=%d", process.processID)
	return nil
}

// Close cleans up any state associated with the process but does not kill
// or wait on it.
func (process *Process) Close() error {
	process.handleLock.Lock()
	defer process.handleLock.Unlock()
	operation := "Close"
	title := "hcsshim::Process::" + operation
	logrus.Debugf(title+" processid=%d", process.processID)

	// Don't double free this
	if process.handle == 0 {
		return nil
	}

	if err := process.unregisterCallback(); err != nil {
		return makeProcessError(process, operation, err, nil)
	}

	if err := hcsCloseProcess(process.handle); err != nil {
		return makeProcessError(process, operation, err, nil)
	}

	process.handle = 0

	logrus.Debugf(title+" succeeded processid=%d", process.processID)
	return nil
}

func (process *Process) registerCallback() error {
	context := &notifcationWatcherContext{
		channels: newChannels(),
	}

	callbackMapLock.Lock()
	callbackNumber := nextCallback
	nextCallback++
	callbackMap[callbackNumber] = context
	callbackMapLock.Unlock()

	var callbackHandle hcsCallback
	err := hcsRegisterProcessCallback(process.handle, notificationWatcherCallback, callbackNumber, &callbackHandle)
	if err != nil {
		return err
	}
	context.handle = callbackHandle
	process.callbackNumber = callbackNumber

	return nil
}

func (process *Process) unregisterCallback() error {
	callbackNumber := process.callbackNumber

	callbackMapLock.RLock()
	context := callbackMap[callbackNumber]
	callbackMapLock.RUnlock()

	if context == nil {
		return nil
	}

	handle := context.handle

	if handle == 0 {
		return nil
	}

	// hcsUnregisterProcessCallback has its own syncronization
	// to wait for all callbacks to complete. We must NOT hold the callbackMapLock.
	err := hcsUnregisterProcessCallback(handle)
	if err != nil {
		return err
	}

	closeChannels(context.channels)

	callbackMapLock.Lock()
	callbackMap[callbackNumber] = nil
	callbackMapLock.Unlock()

	handle = 0

	return nil
}

package hcsshim

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
)

var (
	ErrInvalidProcessState = errors.New("the process is in an invalid state for the attempted operation")
)

type ProcessError struct {
	Process   *process
	Operation string
	Err       error
}

type process struct {
	handle         hcsProcess
	processID      int
	container      *container
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

type processStatus struct {
	ProcessId      uint32
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
func (process *process) Pid() int {
	return process.processID
}

// Kill signals the process to terminate but does not wait for it to finish terminating.
func (process *process) Kill() error {
	operation := "Kill"
	title := "HCSShim::Process::" + operation
	logrus.Debugf(title+" processid=%d", process.processID)

	var resultp *uint16
	err := hcsTerminateProcess(process.handle, &resultp)
	err = processHcsResult(err, resultp)
	if err == ErrVmcomputeOperationPending {
		return ErrVmcomputeOperationPending
	} else if err != nil {
		err := &ProcessError{Operation: operation, Process: process, Err: err}
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+" succeeded processid=%d", process.processID)
	return nil
}

// Wait waits for the process to exit.
func (process *process) Wait() error {
	operation := "Wait"
	title := "HCSShim::Process::" + operation
	logrus.Debugf(title+" processid=%d", process.processID)

	if hcsCallbacksSupported {
		err := waitForNotification(process.callbackNumber, hcsNotificationProcessExited, nil)
		if err != nil {
			err := &ProcessError{Operation: operation, Process: process, Err: err}
			logrus.Error(err)
			return err
		}
	} else {
		_, err := process.waitTimeoutInternal(syscall.INFINITE)
		if err != nil {
			err := &ProcessError{Operation: operation, Process: process, Err: err}
			logrus.Error(err)
			return err
		}
	}

	logrus.Debugf(title+" succeeded processid=%d", process.processID)
	return nil
}

// WaitTimeout waits for the process to exit or the duration to elapse. It returns
// false if timeout occurs.
func (process *process) WaitTimeout(timeout time.Duration) error {
	operation := "WaitTimeout"
	title := "HCSShim::Process::" + operation
	logrus.Debugf(title+" processid=%d", process.processID)

	if hcsCallbacksSupported {
		err := waitForNotification(process.callbackNumber, hcsNotificationProcessExited, &timeout)
		if err == ErrTimeout {
			return ErrTimeout
		} else if err != nil {
			err := &ProcessError{Operation: operation, Process: process, Err: err}
			logrus.Error(err)
			return err
		}
	} else {
		finished, err := waitTimeoutHelper(process, timeout)
		if !finished {
			return ErrTimeout
		} else if err != nil {
			err := &ProcessError{Operation: operation, Process: process, Err: err}
			logrus.Error(err)
			return err
		}
	}

	logrus.Debugf(title+" succeeded processid=%d", process.processID)
	return nil
}

func (process *process) hcsWait(timeout uint32) (bool, error) {
	var (
		resultp   *uint16
		exitEvent syscall.Handle
	)
	err := hcsCreateProcessWait(process.handle, &exitEvent, &resultp)
	err = processHcsResult(err, resultp)
	if err != nil {
		return false, err
	}
	defer syscall.CloseHandle(exitEvent)

	return waitForSingleObject(exitEvent, timeout)
}

func (process *process) waitTimeoutInternal(timeout uint32) (bool, error) {
	return waitTimeoutInternalHelper(process, timeout)
}

// ExitCode returns the exit code of the process. The process must have
// already terminated.
func (process *process) ExitCode() (int, error) {
	operation := "ExitCode"
	title := "HCSShim::Process::" + operation
	logrus.Debugf(title+" processid=%d", process.processID)

	properties, err := process.properties()
	if err != nil {
		err := &ProcessError{Operation: operation, Process: process, Err: err}
		logrus.Error(err)
		return 0, err
	}

	if properties.Exited == false {
		return 0, ErrInvalidProcessState
	}

	logrus.Debugf(title+" succeeded processid=%d exitCode=%d", process.processID, properties.ExitCode)
	return int(properties.ExitCode), nil
}

// ResizeConsole resizes the console of the process.
func (process *process) ResizeConsole(width, height uint16) error {
	operation := "ResizeConsole"
	title := "HCSShim::Process::" + operation
	logrus.Debugf(title+" processid=%d", process.processID)

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
	err = processHcsResult(err, resultp)
	if err != nil {
		err := &ProcessError{Operation: operation, Process: process, Err: err}
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+" succeeded processid=%d", process.processID)
	return nil
}

func (process *process) properties() (*processStatus, error) {
	operation := "properties"
	title := "HCSShim::Process::" + operation
	logrus.Debugf(title+" processid=%d", process.processID)

	var (
		resultp     *uint16
		propertiesp *uint16
	)
	err := hcsGetProcessProperties(process.handle, &propertiesp, &resultp)
	err = processHcsResult(err, resultp)
	if err != nil {
		err := &ProcessError{Operation: operation, Process: process, Err: err}
		logrus.Error(err)
		return nil, err
	}

	if propertiesp == nil {
		return nil, errors.New("Unexpected result from hcsGetProcessProperties, properties should never be nil")
	}
	propertiesRaw := convertAndFreeCoTaskMemBytes(propertiesp)

	properties := &processStatus{}
	if err := json.Unmarshal(propertiesRaw, properties); err != nil {
		return nil, err
	}

	logrus.Debugf(title+" succeeded processid=%d, properties=%s", process.processID, propertiesRaw)
	return properties, nil
}

// Stdio returns the stdin, stdout, and stderr pipes, respectively. Closing
// these pipes does not close the underlying pipes; it should be possible to
// call this multiple times to get multiple interfaces.
func (process *process) Stdio() (io.WriteCloser, io.ReadCloser, io.ReadCloser, error) {
	operation := "Stdio"
	title := "HCSShim::Process::" + operation
	logrus.Debugf(title+" processid=%d", process.processID)

	var stdIn, stdOut, stdErr syscall.Handle

	if process.cachedPipes == nil {
		var (
			processInfo hcsProcessInformation
			resultp     *uint16
		)
		err := hcsGetProcessInfo(process.handle, &processInfo, &resultp)
		err = processHcsResult(err, resultp)
		if err != nil {
			err = &ProcessError{Operation: operation, Process: process, Err: err}
			logrus.Error(err)
			return nil, nil, nil, err
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
		return nil, nil, nil, err
	}

	logrus.Debugf(title+" succeeded processid=%d", process.processID)
	return pipes[0], pipes[1], pipes[2], nil
}

// CloseStdin closes the write side of the stdin pipe so that the process is
// notified on the read side that there is no more data in stdin.
func (process *process) CloseStdin() error {
	operation := "CloseStdin"
	title := "HCSShim::Process::" + operation
	logrus.Debugf(title+" processid=%d", process.processID)

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
	err = processHcsResult(err, resultp)
	if err != nil {
		err = &ProcessError{Operation: operation, Process: process, Err: err}
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+" succeeded processid=%d", process.processID)
	return nil
}

// Close cleans up any state associated with the process but does not kill
// or wait on it.
func (process *process) Close() error {
	operation := "Close"
	title := "HCSShim::Process::" + operation
	logrus.Debugf(title+" processid=%d", process.processID)

	// Don't double free this
	if process.handle == 0 {
		return nil
	}

	if hcsCallbacksSupported {
		if err := process.unregisterCallback(); err != nil {
			err = &ProcessError{Operation: operation, Process: process, Err: err}
			logrus.Error(err)
			return err
		}
	}

	if err := hcsCloseProcess(process.handle); err != nil {
		err = &ProcessError{Operation: operation, Process: process, Err: err}
		logrus.Error(err)
		return err
	}

	process.handle = 0

	logrus.Debugf(title+" succeeded processid=%d", process.processID)
	return nil
}

// closeProcess wraps process.Close for use by a finalizer
func closeProcess(process *process) {
	process.Close()
}

func (process *process) registerCallback() error {
	callbackMapLock.Lock()
	defer callbackMapLock.Unlock()

	callbackNumber := nextCallback
	nextCallback++

	context := &notifcationWatcherContext{
		channels: newChannels(),
	}
	callbackMap[callbackNumber] = context

	var callbackHandle hcsCallback
	err := hcsRegisterProcessCallback(process.handle, notificationWatcherCallback, callbackNumber, &callbackHandle)
	if err != nil {
		return err
	}
	context.handle = callbackHandle
	process.callbackNumber = callbackNumber

	return nil
}

func (process *process) unregisterCallback() error {
	callbackNumber := process.callbackNumber

	callbackMapLock.Lock()
	defer callbackMapLock.Unlock()
	handle := callbackMap[callbackNumber].handle

	if handle == 0 {
		return nil
	}

	err := hcsUnregisterProcessCallback(handle)
	if err != nil {
		return err
	}

	callbackMap[callbackNumber] = nil

	handle = 0

	return nil
}

func (e *ProcessError) Error() string {
	if e == nil {
		return "<nil>"
	}

	if e.Process == nil {
		return "Unexpected nil process for error: " + e.Err.Error()
	}

	s := fmt.Sprintf("process %d", e.Process.processID)

	if e.Process.container != nil {
		s += " in container " + e.Process.container.id
	}

	if e.Operation != "" {
		s += " " + e.Operation
	}

	if e.Err != nil {
		s += fmt.Sprintf(" failed in Win32: %s (0x%x)", e.Err, win32FromError(e.Err))
	}

	return s
}

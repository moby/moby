package hcsshim

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
)

var (
	defaultTimeout = time.Minute * 4

	// ErrTimeout is an error encountered when waiting on a notification times out
	ErrTimeout = errors.New("hcsshim: timeout waiting for notification")
)

type ContainerError struct {
	Container *container
	Operation string
	ExtraInfo string
	Err       error
}

type container struct {
	handle hcsSystem
	id     string
}

type containerProperties struct {
	ID                string `json:"Id"`
	Name              string
	SystemType        string
	Owner             string
	SiloGUID          string `json:"SiloGuid,omitempty"`
	IsDummy           bool   `json:",omitempty"`
	RuntimeID         string `json:"RuntimeId,omitempty"`
	Stopped           bool   `json:",omitempty"`
	ExitType          string `json:",omitempty"`
	AreUpdatesPending bool   `json:",omitempty"`
}

// CreateContainer creates a new container with the given configuration but does not start it.
func CreateContainer(id string, c *ContainerConfig) (Container, error) {
	operation := "CreateContainer"
	title := "HCSShim::" + operation
	logrus.Debugf(title+" id=%s", id)

	container := &container{
		id: id,
	}

	configurationb, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	configuration := string(configurationb)

	var (
		handle      hcsSystem
		resultp     *uint16
		createError error
	)
	if hcsCallbacksSupported {
		var identity syscall.Handle
		createError = hcsCreateComputeSystem(id, configuration, identity, &handle, &resultp)
	} else {
		createError = hcsCreateComputeSystemTP5(id, configuration, &handle, &resultp)
	}
	container.handle = handle

	err = processAsyncHcsResult(container, createError, resultp, hcsNotificationSystemCreateCompleted, &defaultTimeout)
	if err != nil {
		err := &ContainerError{Container: container, Operation: operation, ExtraInfo: configuration, Err: err}
		logrus.Error(err)
		return nil, err
	}

	logrus.Debugf(title+" succeeded id=%s handle=%d", id, container.handle)
	runtime.SetFinalizer(container, closeContainer)
	return container, nil
}

// OpenContainer opens an existing container by ID.
func OpenContainer(id string) (Container, error) {
	operation := "OpenContainer"
	title := "HCSShim::" + operation
	logrus.Debugf(title+" id=%s", id)

	container := &container{
		id: id,
	}

	var (
		handle  hcsSystem
		resultp *uint16
	)
	err := hcsOpenComputeSystem(id, &handle, &resultp)
	err = processHcsResult(err, resultp)
	if err != nil {
		err = &ContainerError{Container: container, Operation: operation, Err: err}
		logrus.Error(err)
		return nil, err
	}

	container.handle = handle

	logrus.Debugf(title+" succeeded id=%s handle=%d", id, handle)
	runtime.SetFinalizer(container, closeContainer)
	return container, nil
}

// Start synchronously starts the container.
func (container *container) Start() error {
	operation := "Start"
	title := "HCSShim::Container::" + operation
	logrus.Debugf(title+" id=%s", container.id)

	var resultp *uint16
	err := hcsStartComputeSystemTP5(container.handle, nil, &resultp)
	err = processAsyncHcsResult(container, err, resultp, hcsNotificationSystemStartCompleted, &defaultTimeout)
	if err != nil {
		err := &ContainerError{Container: container, Operation: operation, Err: err}
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+" succeeded id=%s", container.id)
	return nil
}

// Shutdown requests a container shutdown, but it may not actually be shut down until Wait() succeeds.
// It returns ErrVmcomputeOperationPending if the shutdown is in progress, nil if the shutdown is complete.
func (container *container) Shutdown() error {
	operation := "Shutdown"
	title := "HCSShim::Container::" + operation
	logrus.Debugf(title+" id=%s", container.id)

	var resultp *uint16
	err := hcsShutdownComputeSystemTP5(container.handle, nil, &resultp)
	err = processHcsResult(err, resultp)
	if err != nil {
		if err == ErrVmcomputeOperationPending {
			return ErrVmcomputeOperationPending
		}
		err = &ContainerError{Container: container, Operation: operation, Err: err}
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+" succeeded id=%s", container.id)
	return nil
}

// Terminate requests a container terminate, but it may not actually be terminated until Wait() succeeds.
// It returns ErrVmcomputeOperationPending if the shutdown is in progress, nil if the shutdown is complete.
func (container *container) Terminate() error {
	operation := "Terminate"
	title := "HCSShim::Container::" + operation
	logrus.Debugf(title+" id=%s", container.id)

	var resultp *uint16
	err := hcsTerminateComputeSystemTP5(container.handle, nil, &resultp)
	err = processHcsResult(err, resultp)
	if err != nil {
		if err == ErrVmcomputeOperationPending {
			return ErrVmcomputeOperationPending
		}
		err = &ContainerError{Container: container, Operation: operation, Err: err}
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+" succeeded id=%s", container.id)
	return nil
}

// Wait synchronously waits for the container to shutdown or terminate.
func (container *container) Wait() error {
	operation := "Wait"
	title := "HCSShim::Container::" + operation
	logrus.Debugf(title+" id=%s", container.id)

	if hcsCallbacksSupported {
		err := registerAndWaitForCallback(container, hcsNotificationSystemExited)
		if err != nil {
			err := &ContainerError{Container: container, Operation: operation, Err: err}
			logrus.Error(err)
			return err
		}
	} else {
		_, err := container.waitTimeoutInternal(syscall.INFINITE)
		if err != nil {
			err := &ContainerError{Container: container, Operation: operation, Err: err}
			logrus.Error(err)
			return err
		}
	}

	logrus.Debugf(title+" succeeded id=%s", container.id)
	return nil
}

func (container *container) waitTimeoutInternal(timeout uint32) (bool, error) {
	return waitTimeoutInternalHelper(container, timeout)
}

// WaitTimeout synchronously waits for the container to terminate or the duration to elapse. It returns
// ErrTimeout if the timeout duration expires before the container is shut down.
func (container *container) WaitTimeout(timeout time.Duration) error {
	operation := "WaitTimeout"
	title := "HCSShim::Container::" + operation
	logrus.Debugf(title+" id=%s", container.id)

	if hcsCallbacksSupported {
		err := registerAndWaitForCallbackTimeout(container, hcsNotificationSystemExited, timeout)
		if err == ErrTimeout {
			return ErrTimeout
		} else if err != nil {
			err := &ContainerError{Container: container, Operation: operation, Err: err}
			logrus.Error(err)
			return err
		}
	} else {
		finished, err := waitTimeoutHelper(container, timeout)
		if !finished {
			return ErrTimeout
		} else if err != nil {
			err := &ContainerError{Container: container, Operation: operation, Err: err}
			logrus.Error(err)
			return err
		}
	}

	logrus.Debugf(title+" succeeded id=%s", container.id)
	return nil
}

func (container *container) hcsWait(timeout uint32) (bool, error) {
	var (
		resultp   *uint16
		exitEvent syscall.Handle
	)

	err := hcsCreateComputeSystemWait(container.handle, &exitEvent, &resultp)
	err = processHcsResult(err, resultp)
	if err != nil {
		return false, err
	}
	defer syscall.CloseHandle(exitEvent)

	return waitForSingleObject(exitEvent, timeout)
}

func (container *container) properties() (*containerProperties, error) {
	var (
		resultp     *uint16
		propertiesp *uint16
	)
	err := hcsGetComputeSystemProperties(container.handle, "", &propertiesp, &resultp)
	err = processHcsResult(err, resultp)
	if err != nil {
		return nil, err
	}

	if propertiesp == nil {
		return nil, errors.New("Unexpected result from hcsGetComputeSystemProperties, properties should never be nil")
	}
	propertiesRaw := convertAndFreeCoTaskMemBytes(propertiesp)

	properties := &containerProperties{}
	if err := json.Unmarshal(propertiesRaw, properties); err != nil {
		return nil, err
	}

	return properties, nil
}

// HasPendingUpdates returns true if the container has updates pending to install
func (container *container) HasPendingUpdates() (bool, error) {
	operation := "HasPendingUpdates"
	title := "HCSShim::Container::" + operation
	logrus.Debugf(title+" id=%s", container.id)
	properties, err := container.properties()
	if err != nil {
		err := &ContainerError{Container: container, Operation: operation, Err: err}
		logrus.Error(err)
		return false, err
	}

	logrus.Debugf(title+" succeeded id=%s", container.id)
	return properties.AreUpdatesPending, nil
}

// Pause pauses the execution of the container. This feature is not enabled in TP5.
func (container *container) Pause() error {
	operation := "Pause"
	title := "HCSShim::Container::" + operation
	logrus.Debugf(title+" id=%s", container.id)

	var resultp *uint16
	err := hcsPauseComputeSystemTP5(container.handle, nil, &resultp)
	err = processAsyncHcsResult(container, err, resultp, hcsNotificationSystemPauseCompleted, &defaultTimeout)
	if err != nil {
		err := &ContainerError{Container: container, Operation: operation, Err: err}
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+" succeeded id=%s", container.id)
	return nil
}

// Resume resumes the execution of the container. This feature is not enabled in TP5.
func (container *container) Resume() error {
	operation := "Resume"
	title := "HCSShim::Container::" + operation
	logrus.Debugf(title+" id=%s", container.id)
	var (
		resultp *uint16
	)

	err := hcsResumeComputeSystemTP5(container.handle, nil, &resultp)
	err = processAsyncHcsResult(container, err, resultp, hcsNotificationSystemResumeCompleted, &defaultTimeout)
	if err != nil {
		err := &ContainerError{Container: container, Operation: operation, Err: err}
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+" succeeded id=%s", container.id)
	return nil
}

// CreateProcess launches a new process within the container.
func (container *container) CreateProcess(c *ProcessConfig) (Process, error) {
	operation := "CreateProcess"
	title := "HCSShim::Container::" + operation
	logrus.Debugf(title+" id=%s", container.id)
	var (
		processInfo   hcsProcessInformation
		processHandle hcsProcess
		resultp       *uint16
	)

	// If we are not emulating a console, ignore any console size passed to us
	if !c.EmulateConsole {
		c.ConsoleSize[0] = 0
		c.ConsoleSize[1] = 0
	}

	configurationb, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	configuration := string(configurationb)

	err = hcsCreateProcess(container.handle, configuration, &processInfo, &processHandle, &resultp)
	err = processHcsResult(err, resultp)
	if err != nil {
		err = &ContainerError{Container: container, Operation: operation, ExtraInfo: configuration, Err: err}
		logrus.Error(err)
		return nil, err
	}

	process := &process{
		handle:    processHandle,
		processID: int(processInfo.ProcessId),
		container: container,
		cachedPipes: &cachedPipes{
			stdIn:  processInfo.StdInput,
			stdOut: processInfo.StdOutput,
			stdErr: processInfo.StdError,
		},
	}

	logrus.Debugf(title+" succeeded id=%s processid=%s", container.id, process.processID)
	runtime.SetFinalizer(process, closeProcess)
	return process, nil
}

// OpenProcess gets an interface to an existing process within the container.
func (container *container) OpenProcess(pid int) (Process, error) {
	operation := "OpenProcess"
	title := "HCSShim::Container::" + operation
	logrus.Debugf(title+" id=%s, processid=%d", container.id, pid)
	var (
		processHandle hcsProcess
		resultp       *uint16
	)

	err := hcsOpenProcess(container.handle, uint32(pid), &processHandle, &resultp)
	err = processHcsResult(err, resultp)
	if err != nil {
		err = &ContainerError{Container: container, Operation: operation, Err: err}
		logrus.Error(err)
		return nil, err
	}

	process := &process{
		handle:    processHandle,
		processID: pid,
		container: container,
	}

	logrus.Debugf(title+" succeeded id=%s processid=%s", container.id, process.processID)
	runtime.SetFinalizer(process, closeProcess)
	return process, nil
}

// Close cleans up any state associated with the container but does not terminate or wait for it.
func (container *container) Close() error {
	operation := "Close"
	title := "HCSShim::Container::" + operation
	logrus.Debugf(title+" id=%s", container.id)

	// Don't double free this
	if container.handle == 0 {
		return nil
	}

	if err := hcsCloseComputeSystem(container.handle); err != nil {
		err = &ContainerError{Container: container, Operation: operation, Err: err}
		logrus.Error(err)
		return err
	}

	container.handle = 0

	logrus.Debugf(title+" succeeded id=%s", container.id)
	return nil
}

// closeContainer wraps container.Close for use by a finalizer
func closeContainer(container *container) {
	container.Close()
}

func (container *container) registerCallback(expectedNotification hcsNotification) (uintptr, error) {
	callbackMapLock.Lock()
	defer callbackMapLock.Unlock()

	callbackNumber := nextCallback
	nextCallback++

	context := &notifcationWatcherContext{
		expectedNotification: expectedNotification,
		channel:              make(chan error, 1),
	}
	callbackMap[callbackNumber] = context

	var callbackHandle hcsCallback
	err := hcsRegisterComputeSystemCallback(container.handle, notificationWatcherCallback, callbackNumber, &callbackHandle)
	if err != nil {
		return 0, err
	}
	context.handle = callbackHandle

	return callbackNumber, nil
}

func (container *container) unregisterCallback(callbackNumber uintptr) error {
	callbackMapLock.Lock()
	defer callbackMapLock.Unlock()

	handle := callbackMap[callbackNumber].handle

	if handle == 0 {
		return nil
	}

	err := hcsUnregisterComputeSystemCallback(handle)
	if err != nil {
		return err
	}

	callbackMap[callbackNumber] = nil

	handle = 0

	return nil
}

func (e *ContainerError) Error() string {
	if e == nil {
		return "<nil>"
	}

	if e.Container == nil {
		return "unexpected nil container for error: " + e.Err.Error()
	}

	s := "container " + e.Container.id

	if e.Operation != "" {
		s += " encountered an error during " + e.Operation
	}

	if e.Err != nil {
		s += fmt.Sprintf(" failed in Win32: %s (0x%x)", e.Err, win32FromError(e.Err))
	}

	if e.ExtraInfo != "" {
		s += " extra info: " + e.ExtraInfo
	}

	return s
}

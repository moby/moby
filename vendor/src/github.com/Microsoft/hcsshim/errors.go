package hcsshim

import (
	"errors"
	"fmt"
	"syscall"

	"github.com/Sirupsen/logrus"
)

var (
	// ErrHandleClose is an error returned when the handle generating the notification being waited on has been closed
	ErrHandleClose = errors.New("hcsshim: the handle generating this notification has been closed")

	// ErrInvalidNotificationType is an error encountered when an invalid notification type is used
	ErrInvalidNotificationType = errors.New("hcsshim: invalid notification type")

	// ErrInvalidProcessState is an error encountered when the process is not in a valid state for the requested operation
	ErrInvalidProcessState = errors.New("the process is in an invalid state for the attempted operation")

	// ErrTimeout is an error encountered when waiting on a notification times out
	ErrTimeout = errors.New("hcsshim: timeout waiting for notification")

	// ErrUnexpectedContainerExit is the error returned when a container exits while waiting for
	// a different expected notification
	ErrUnexpectedContainerExit = errors.New("unexpected container exit")

	// ErrUnexpectedProcessAbort is the error returned when communication with the compute service
	// is lost while waiting for a notification
	ErrUnexpectedProcessAbort = errors.New("lost communication with compute service")

	// ErrUnexpectedValue is an error returned when hcs returns an invalid value
	ErrUnexpectedValue = errors.New("unexpected value returned from hcs")

	// ErrVmcomputeAlreadyStopped is an error returned when a shutdown or terminate request is made on a stopped container
	ErrVmcomputeAlreadyStopped = syscall.Errno(0xc0370110)

	// ErrVmcomputeOperationPending is an error returned when the operation is being completed asynchronously
	ErrVmcomputeOperationPending = syscall.Errno(0xC0370103)
)

// ProcessError is an error encountered in HCS during an operation on a Process object
type ProcessError struct {
	Process   *process
	Operation string
	ExtraInfo string
	Err       error
}

// ContainerError is an error encountered in HCS during an operation on a Container object
type ContainerError struct {
	Container *container
	Operation string
	ExtraInfo string
	Err       error
}

func isKnownError(err error) bool {
	// Don't wrap errors created in hcsshim
	if err == ErrHandleClose ||
		err == ErrInvalidNotificationType ||
		err == ErrInvalidProcessState ||
		err == ErrTimeout ||
		err == ErrUnexpectedContainerExit ||
		err == ErrUnexpectedProcessAbort ||
		err == ErrUnexpectedValue ||
		err == ErrVmcomputeAlreadyStopped ||
		err == ErrVmcomputeOperationPending {
		return true
	}

	return false
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

func makeContainerError(container *container, operation string, extraInfo string, err error) error {
	// Return known errors to the client
	if isKnownError(err) {
		return err
	}

	// Log any unexpected errors
	containerError := &ContainerError{Container: container, Operation: operation, ExtraInfo: extraInfo, Err: err}
	logrus.Error(containerError)
	return containerError
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

func makeProcessError(process *process, operation string, extraInfo string, err error) error {
	// Return known errors to the client
	if isKnownError(err) {
		return err
	}

	// Log any unexpected errors
	processError := &ProcessError{Process: process, Operation: operation, ExtraInfo: extraInfo, Err: err}
	logrus.Error(processError)
	return processError
}

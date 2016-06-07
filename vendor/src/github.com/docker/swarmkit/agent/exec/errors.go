package exec

import (
	"errors"
	"fmt"

	"github.com/docker/swarmkit/api"
)

var (
	// ErrRuntimeUnsupported encountered when a task requires a runtime
	// unsupported by the executor.
	ErrRuntimeUnsupported = errors.New("exec: unsupported runtime")

	// ErrTaskPrepared is called if the task is already prepared.
	ErrTaskPrepared = errors.New("exec: task already prepared")

	// ErrTaskStarted can be returned from any operation that cannot be
	// performed because the task has already been started. This does not imply
	// that the task is running but rather that it is no longer valid to call
	// Start.
	ErrTaskStarted = errors.New("exec: task already started")

	// ErrTaskUpdateFailed is returned if a task controller update fails.
	ErrTaskUpdateFailed = errors.New("exec: task update failed")

	// ErrControllerClosed returned when a task controller has been closed.
	ErrControllerClosed = errors.New("exec: controller closed")
)

// Temporary indicates whether or not the error condition is temporary.
//
// If this is encountered in the controller, the failing operation will be
// retried when this returns true. Otherwise, the operation is considered
// fatal.
type Temporary interface {
	Temporary() bool
}

// ExitError is returned by controller methods after encountering an error after a
// task exits. It should require any data to report on a non-zero exit code.
type ExitError struct {
	Code            int
	Cause           error
	ContainerStatus *api.ContainerStatus
}

func (e *ExitError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("task: non-zero exit (%v): %v", e.Code, e.Cause)
	}

	return fmt.Sprintf("task: non-zero exit (%v)", e.Code)
}

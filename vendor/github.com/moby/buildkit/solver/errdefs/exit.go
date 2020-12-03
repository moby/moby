package errdefs

import "fmt"

const (
	// ContainerdUnknownExitStatus is returned when containerd is unable to
	// determine the exit status of a process. This can happen if the process never starts
	// or if an error was encountered when obtaining the exit status, it is set to 255.
	//
	// This const is defined here to prevent importing github.com/containerd/containerd
	// and corresponds with https://github.com/containerd/containerd/blob/40b22ef0741028917761d8c5d5d29e0d19038836/task.go#L52-L55
	ContainerdUnknownExitStatus = 255
)

// ExitError will be returned when the container process exits with a non-zero
// exit code.
type ExitError struct {
	ExitCode uint32
	Err      error
}

func (err *ExitError) Error() string {
	if err.Err != nil {
		return err.Err.Error()
	}
	return fmt.Sprintf("exit code: %d", err.ExitCode)
}

func (err *ExitError) Unwrap() error {
	if err.Err == nil {
		return fmt.Errorf("exit code: %d", err.ExitCode)
	}
	return err.Err
}

package errdefs

import fmt "fmt"

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

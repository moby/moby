package container

// StateStatus is used to return container wait results.
// Implements exec.ExitCode interface.
// This type is needed as State include a sync.Mutex field which make
// copying it unsafe.
type StateStatus struct {
	exitCode int
	err      error
}

// ExitCode returns current exitcode for the state.
func (s StateStatus) ExitCode() int {
	return s.exitCode
}

// Err returns current error for the state. Returns nil if the container had
// exited on its own.
func (s StateStatus) Err() error {
	return s.err
}

// NewStateStatus returns a new StateStatus with the given exit code and error.
func NewStateStatus(exitCode int, err error) StateStatus {
	return StateStatus{
		exitCode: exitCode,
		err:      err,
	}
}

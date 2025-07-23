package container

import "errors"

// Err returns the error message, if any.
func (e *WaitResponse) Err() error {
	if e == nil || e.Error == nil || e.Error.Message == "" {
		return nil
	}
	return errors.New(e.Error.Message)
}

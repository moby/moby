package container

import "errors"

// Err returns the error message, if any.
func (w *WaitResponse) Err() error {
	if w == nil || w.Error == nil || w.Error.Message == "" {
		return nil
	}
	return errors.New(w.Error.Message)
}

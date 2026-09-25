package runconfig

import cerrdefs "github.com/containerd/errdefs"

func validationError(msg string) error {
	return cerrdefs.ErrInvalidArgument.WithMessage(msg)
}

type invalidJSONError struct{ error }

func (e invalidJSONError) Error() string {
	return "invalid JSON: " + e.error.Error()
}

func (e invalidJSONError) Unwrap() error {
	return e.error
}

func (e invalidJSONError) InvalidParameter() {}

package errdefs

import (
	serrdefs "github.com/moby/buildkit/solver/errdefs"
)

// FileActionError will be returned when an error is encountered when solving
// a fileop.
type FileActionError struct {
	error
	Index int
}

func (e *FileActionError) Unwrap() error {
	return e.error
}

func (e *FileActionError) ToSubject() serrdefs.IsSolve_Subject {
	return &serrdefs.Solve_File{
		File: &serrdefs.FileAction{
			Index: int64(e.Index),
		},
	}
}

func WithFileActionError(err error, idx int) error {
	if err == nil {
		return nil
	}
	return &FileActionError{
		error: err,
		Index: idx,
	}
}

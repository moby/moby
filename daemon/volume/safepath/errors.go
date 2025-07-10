package safepath

// ErrNotAccessible is returned by Join when the resulting path doesn't exist,
// is not accessible, or any of the path components was replaced with a symlink
// during the path traversal.
type ErrNotAccessible struct {
	Path  string
	Cause error
}

func (*ErrNotAccessible) NotFound() {}

func (e *ErrNotAccessible) Unwrap() error {
	return e.Cause
}

func (e *ErrNotAccessible) Error() string {
	msg := "cannot access path " + e.Path
	if e.Cause != nil {
		msg += ": " + e.Cause.Error()
	}
	return msg
}

// ErrEscapesBase is returned by Join when the resulting concatenation would
// point outside of the specified base directory.
type ErrEscapesBase struct {
	Base, Subpath string
}

func (*ErrEscapesBase) InvalidParameter() {}

func (e *ErrEscapesBase) Error() string {
	msg := "path concatenation escapes the base directory"
	if e.Base != "" {
		msg += ", base: " + e.Base
	}
	if e.Subpath != "" {
		msg += ", subpath: " + e.Subpath
	}
	return msg
}

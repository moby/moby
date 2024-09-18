package system // import "github.com/docker/docker/pkg/system"

type XattrError struct {
	Op   string
	Attr string
	Path string
	Err  error
}

func (e *XattrError) Error() string { return e.Op + " " + e.Attr + " " + e.Path + ": " + e.Err.Error() }

func (e *XattrError) Unwrap() error { return e.Err }

// Timeout reports whether this error represents a timeout.
func (e *XattrError) Timeout() bool {
	t, ok := e.Err.(interface{ Timeout() bool })
	return ok && t.Timeout()
}

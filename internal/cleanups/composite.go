package cleanups

import (
	"github.com/docker/docker/internal/multierror"
)

type Composite struct {
	cleanups []func() error
}

// Add adds a cleanup to be called.
func (c *Composite) Add(f func() error) {
	c.cleanups = append(c.cleanups, f)
}

// Call calls all cleanups in reverse order and returns an error combining all
// non-nil errors.
func (c *Composite) Call() error {
	err := call(c.cleanups)
	c.cleanups = nil
	return err
}

// Release removes all cleanups, turning Call into a no-op.
// Caller still can call the cleanups by calling the returned function
// which is equivalent to calling the Call before Release was called.
func (c *Composite) Release() func() error {
	cleanups := c.cleanups
	c.cleanups = nil
	return func() error {
		return call(cleanups)
	}
}

func call(cleanups []func() error) error {
	var errs []error
	for idx := len(cleanups) - 1; idx >= 0; idx-- {
		c := cleanups[idx]
		errs = append(errs, c())
	}
	return multierror.Join(errs...)
}

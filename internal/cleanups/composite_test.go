package cleanups

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestCall(t *testing.T) {
	c := Composite{}
	err1 := errors.New("error1")
	err2 := errors.New("error2")
	errX := errors.New("errorX")
	errY := errors.New("errorY")
	errZ := errors.New("errorZ")
	errYZ := errors.Join(errY, errZ)

	c.Add(func(ctx context.Context) error {
		return err1
	})
	c.Add(func(ctx context.Context) error {
		return nil
	})
	c.Add(func(ctx context.Context) error {
		return fmt.Errorf("something happened: %w", err2)
	})
	c.Add(func(ctx context.Context) error {
		return errors.Join(errX, fmt.Errorf("joined: %w", errYZ))
	})

	err := c.Call(context.Background())

	errs := err.(interface{ Unwrap() []error }).Unwrap()

	assert.Check(t, is.ErrorContains(err, err1.Error()))
	assert.Check(t, is.ErrorContains(err, err2.Error()))
	assert.Check(t, is.ErrorContains(err, errX.Error()))
	assert.Check(t, is.ErrorContains(err, errY.Error()))
	assert.Check(t, is.ErrorContains(err, errZ.Error()))
	assert.Check(t, is.ErrorContains(err, "something happened: "+err2.Error()))

	t.Logf(err.Error())
	assert.Assert(t, is.Len(errs, 3))

	// Cleanups executed in reverse order.
	assert.Check(t, is.ErrorIs(errs[2], err1))
	assert.Check(t, is.ErrorIs(errs[1], err2))
	assert.Check(t, is.ErrorIs(errs[0], errX))
	assert.Check(t, is.ErrorIs(errs[0], errYZ))
}

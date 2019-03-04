package opts

import (
	"time"

	"github.com/pkg/errors"
)

// PositiveDurationOpt is an option type for time.Duration that uses a pointer.
// It behave similarly to DurationOpt but only allows positive duration values.
type PositiveDurationOpt struct {
	DurationOpt
}

// Set a new value on the option. Setting a negative duration value will cause
// an error to be returned.
func (d *PositiveDurationOpt) Set(s string) error {
	err := d.DurationOpt.Set(s)
	if err != nil {
		return err
	}
	if *d.DurationOpt.value < 0 {
		return errors.Errorf("duration cannot be negative")
	}
	return nil
}

// DurationOpt is an option type for time.Duration that uses a pointer. This
// allows us to get nil values outside, instead of defaulting to 0
type DurationOpt struct {
	value *time.Duration
}

// NewDurationOpt creates a DurationOpt with the specified duration
func NewDurationOpt(value *time.Duration) *DurationOpt {
	return &DurationOpt{
		value: value,
	}
}

// Set a new value on the option
func (d *DurationOpt) Set(s string) error {
	v, err := time.ParseDuration(s)
	d.value = &v
	return err
}

// Type returns the type of this option, which will be displayed in `--help` output
func (d *DurationOpt) Type() string {
	return "duration"
}

// String returns a string repr of this option
func (d *DurationOpt) String() string {
	if d.value != nil {
		return d.value.String()
	}
	return ""
}

// Value returns the time.Duration
func (d *DurationOpt) Value() *time.Duration {
	return d.value
}

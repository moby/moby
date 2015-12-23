package opts

import (
	"fmt"

	"github.com/docker/docker/api/types/blkiodev"
)

// ThrottledeviceOpt defines a map of ThrottleDevices
type ThrottledeviceOpt struct {
	values    []*blkiodev.ThrottleDevice
	validator ValidatorThrottleFctType
}

// NewThrottledeviceOpt creates a new ThrottledeviceOpt
func NewThrottledeviceOpt(validator ValidatorThrottleFctType) ThrottledeviceOpt {
	values := []*blkiodev.ThrottleDevice{}
	return ThrottledeviceOpt{
		values:    values,
		validator: validator,
	}
}

// Set validates a ThrottleDevice and sets its name as a key in ThrottledeviceOpt
func (opt *ThrottledeviceOpt) Set(val string) error {
	var value *blkiodev.ThrottleDevice
	if opt.validator != nil {
		v, err := opt.validator(val)
		if err != nil {
			return err
		}
		value = v
	}
	(opt.values) = append((opt.values), value)
	return nil
}

// String returns ThrottledeviceOpt values as a string.
func (opt *ThrottledeviceOpt) String() string {
	var out []string
	for _, v := range opt.values {
		out = append(out, v.String())
	}

	return fmt.Sprintf("%v", out)
}

// GetList returns a slice of pointers to ThrottleDevices.
func (opt *ThrottledeviceOpt) GetList() []*blkiodev.ThrottleDevice {
	var throttledevice []*blkiodev.ThrottleDevice
	for _, v := range opt.values {
		throttledevice = append(throttledevice, v)
	}

	return throttledevice
}

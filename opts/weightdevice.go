package opts

import (
	"fmt"

	"github.com/docker/docker/pkg/blkiodev"
)

// WeightdeviceOpt defines a map of WeightDevices
type WeightdeviceOpt struct {
	values    []*blkiodev.WeightDevice
	validator ValidatorWeightFctType
}

// NewWeightdeviceOpt creates a new WeightdeviceOpt
func NewWeightdeviceOpt(validator ValidatorWeightFctType) WeightdeviceOpt {
	values := []*blkiodev.WeightDevice{}
	return WeightdeviceOpt{
		values:    values,
		validator: validator,
	}
}

// Set validates a WeightDevice and sets its name as a key in WeightdeviceOpt
func (opt *WeightdeviceOpt) Set(val string) error {
	var value *blkiodev.WeightDevice
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

// String returns WeightdeviceOpt values as a string.
func (opt *WeightdeviceOpt) String() string {
	var out []string
	for _, v := range opt.values {
		out = append(out, v.String())
	}

	return fmt.Sprintf("%v", out)
}

// GetList returns a slice of pointers to WeightDevices.
func (opt *WeightdeviceOpt) GetList() []*blkiodev.WeightDevice {
	var weightdevice []*blkiodev.WeightDevice
	for _, v := range opt.values {
		weightdevice = append(weightdevice, v)
	}

	return weightdevice
}

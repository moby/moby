package opts

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/blkiodev"
)

// ValidatorWeightFctType defines a validator function that returns a validated struct and/or an error.
type ValidatorWeightFctType func(val string) (*blkiodev.WeightDevice, error)

// ValidateWeightDevice validates that the specified string has a valid device-weight format.
func ValidateWeightDevice(val string) (*blkiodev.WeightDevice, error) {
	split := strings.SplitN(val, ":", 2)
	if len(split) != 2 {
		return nil, fmt.Errorf("bad format: %s", val)
	}
	if !strings.HasPrefix(split[0], "/dev/") {
		return nil, fmt.Errorf("bad format for device path: %s", val)
	}
	weight, err := strconv.ParseUint(split[1], 10, 0)
	if err != nil {
		return nil, fmt.Errorf("invalid weight for device: %s", val)
	}
	if weight > 0 && (weight < 10 || weight > 1000) {
		return nil, fmt.Errorf("invalid weight for device: %s", val)
	}

	return &blkiodev.WeightDevice{
		Path:   split[0],
		Weight: uint16(weight),
	}, nil
}

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

// Type returns the option type
func (opt *WeightdeviceOpt) Type() string {
	return "list"
}

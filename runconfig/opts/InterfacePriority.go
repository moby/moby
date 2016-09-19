package opts

import (
	"fmt"
	"strconv"
	"strings"

	containertypes "github.com/docker/docker/api/types/container"
)

// ValidatorIfPriorityFctType defines a validator function that returns a validated struct and/or an error.
type ValidatorIfPriorityFctType func(val string) (*containertypes.IfPriority, error)

// ValidateIfPriority validates that the specified string has a valid device-rate format.
func ValidateIfPriority(val string) (*containertypes.IfPriority, error) {
	split := strings.SplitN(val, ":", 2)
	if len(split) != 2 {
		return nil, fmt.Errorf("bad format: %s", val)
	}

	prio, err := strconv.ParseUint(split[1], 10, 32)
	if err != nil || prio < 0 {
		return nil, fmt.Errorf("invalid priority for network interface: %s. The correct format is <network-interface>:<number>. Number must be a positive integer", val)
	}

	return &containertypes.IfPriority{
		Name:     split[0],
		Priority: uint32(prio),
	}, nil
}

// IfPriorityOpt defines a map of IfPrioritys
type IfPriorityOpt struct {
	values    []*containertypes.IfPriority
	validator ValidatorIfPriorityFctType
}

// NewIfPriorityOpt creates a new IfPriorityOpt
func NewIfPriorityOpt(validator ValidatorIfPriorityFctType) IfPriorityOpt {
	values := []*containertypes.IfPriority{}
	return IfPriorityOpt{
		values:    values,
		validator: validator,
	}
}

// Set validates a IfPriority and sets its name as a key in IfPriorityOpt
func (opt *IfPriorityOpt) Set(val string) error {
	var value *containertypes.IfPriority
	if opt.validator != nil {
		v, err := opt.validator(val)
		if err != nil {
			return err
		}
		value = v
	}
	opt.values = append(opt.values, value)
	return nil
}

// String returns IfPriorityOpt values as a string.
func (opt *IfPriorityOpt) String() string {
	var out []string
	for _, v := range opt.values {
		out = append(out, v.String())
	}

	return fmt.Sprintf("%v", out)
}

// GetList returns a slice of pointers to IfPrioritys.
func (opt *IfPriorityOpt) GetList() []*containertypes.IfPriority {
	var ifPriorities []*containertypes.IfPriority
	for _, v := range opt.values {
		ifPriorities = append(ifPriorities, v)
	}

	return ifPriorities
}

// Type returns the option type
func (opt *IfPriorityOpt) Type() string {
	return "interface-priority"
}

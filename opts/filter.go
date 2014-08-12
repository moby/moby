package opts

import (
	flag "github.com/docker/docker/pkg/mflag"
)

func Filter(dst flag.Value, validator func(string) (string, error)) flag.Value {
	return &filter{
		Value:     dst,
		validator: validator,
	}
}

type filter struct {
	flag.Value
	validator func(string) (string, error)
}

func (f *filter) Set(val string) error {
	newval, err := f.validator(val)
	if err != nil {
		return err
	}
	return f.Value.Set(newval)
}

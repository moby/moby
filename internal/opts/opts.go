package opts

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/opts"
)

// SetOpts holds a map of values and a validation function.
type SetOpts struct {
	values map[string]bool
}

// Set validates if needed the input value and add it to the
// internal map, by splitting on '='.
func (opts *SetOpts) Set(value string) error {
	k, v, found := strings.Cut(value, "=")
	var isSet bool
	if !found {
		isSet = true
		k = value
	} else {
		var err error
		isSet, err = strconv.ParseBool(v)
		if err != nil {
			return err
		}
	}
	opts.values[k] = isSet
	return nil
}

// GetAll returns the values of SetOpts as a map.
func (opts *SetOpts) GetAll() map[string]bool {
	return opts.values
}

func (opts *SetOpts) String() string {
	return fmt.Sprintf("%v", opts.values)
}

// Type returns a string name for this Option type
func (opts *SetOpts) Type() string {
	return "map"
}

// NewSetOpts creates a new SetOpts with the specified set of values as a map of string to bool.
func NewSetOpts(values map[string]bool) *SetOpts {
	if values == nil {
		values = make(map[string]bool)
	}
	return &SetOpts{
		values: values,
	}
}

// NamedSetOpts is a SetOpts struct with a configuration name.
// This struct is useful to keep reference to the assigned
// field name in the internal configuration struct.
type NamedSetOpts struct {
	SetOpts
	name string
}

var _ opts.NamedOption = &NamedSetOpts{}

// NewNamedSetOpts creates a reference to a new NamedSetOpts struct.
func NewNamedSetOpts(name string, values map[string]bool) *NamedSetOpts {
	return &NamedSetOpts{
		SetOpts: *NewSetOpts(values),
		name:    name,
	}
}

// Name returns the name of the NamedSetOpts in the configuration.
func (o *NamedSetOpts) Name() string {
	return o.name
}

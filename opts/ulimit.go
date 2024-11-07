package opts // import "github.com/docker/docker/opts"

import (
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-units"
)

// UlimitOpt defines a map of Ulimits
type UlimitOpt struct {
	values *map[string]*container.Ulimit
}

// NewUlimitOpt creates a new UlimitOpt
func NewUlimitOpt(ref *map[string]*container.Ulimit) *UlimitOpt {
	// TODO(thaJeztah): why do we need a map with pointers here?
	if ref == nil {
		ref = &map[string]*container.Ulimit{}
	}
	return &UlimitOpt{ref}
}

// Set validates a Ulimit and sets its name as a key in UlimitOpt
func (o *UlimitOpt) Set(val string) error {
	// FIXME(thaJeztah): these functions also need to be moved over from go-units.
	l, err := units.ParseUlimit(val)
	if err != nil {
		return err
	}

	(*o.values)[l.Name] = l

	return nil
}

// String returns Ulimit values as a string.
func (o *UlimitOpt) String() string {
	var out []string
	for _, v := range *o.values {
		out = append(out, v.String())
	}

	return fmt.Sprintf("%v", out)
}

// GetList returns a slice of pointers to Ulimits.
func (o *UlimitOpt) GetList() []*container.Ulimit {
	var ulimits []*container.Ulimit
	for _, v := range *o.values {
		ulimits = append(ulimits, v)
	}

	return ulimits
}

// Type returns the option type
func (o *UlimitOpt) Type() string {
	return "ulimit"
}

// NamedUlimitOpt defines a named map of Ulimits
type NamedUlimitOpt struct {
	name string
	UlimitOpt
}

var _ NamedOption = &NamedUlimitOpt{}

// NewNamedUlimitOpt creates a new NamedUlimitOpt
func NewNamedUlimitOpt(name string, ref *map[string]*container.Ulimit) *NamedUlimitOpt {
	if ref == nil {
		ref = &map[string]*container.Ulimit{}
	}
	return &NamedUlimitOpt{
		name:      name,
		UlimitOpt: *NewUlimitOpt(ref),
	}
}

// Name returns the option name
func (o *NamedUlimitOpt) Name() string {
	return o.name
}

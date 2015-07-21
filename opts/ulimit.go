package opts

import (
	"fmt"

	"github.com/docker/docker/pkg/ulimit"
)

// UlimitOpt defines a map of Ulimits
type UlimitOpt struct {
	values *map[string]*ulimit.Ulimit
}

// NewUlimitOpt creates a new UlimitOpt
func NewUlimitOpt(ref *map[string]*ulimit.Ulimit) *UlimitOpt {
	if ref == nil {
		ref = &map[string]*ulimit.Ulimit{}
	}
	return &UlimitOpt{ref}
}

// Set validates a Ulimit and sets its name as a key in UlimitOpt
func (o *UlimitOpt) Set(val string) error {
	l, err := ulimit.Parse(val)
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
func (o *UlimitOpt) GetList() []*ulimit.Ulimit {
	var ulimits []*ulimit.Ulimit
	for _, v := range *o.values {
		ulimits = append(ulimits, v)
	}

	return ulimits
}

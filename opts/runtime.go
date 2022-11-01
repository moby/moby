package opts // import "github.com/docker/docker/opts"

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
)

// RuntimeOpt defines a map of Runtimes
type RuntimeOpt struct {
	name             string
	stockRuntimeName string
	values           *map[string]types.Runtime
}

// NewNamedRuntimeOpt creates a new RuntimeOpt
func NewNamedRuntimeOpt(name string, ref *map[string]types.Runtime, stockRuntime string) *RuntimeOpt {
	if ref == nil {
		ref = &map[string]types.Runtime{}
	}
	return &RuntimeOpt{name: name, values: ref, stockRuntimeName: stockRuntime}
}

// Name returns the name of the NamedListOpts in the configuration.
func (o *RuntimeOpt) Name() string {
	return o.name
}

// Set validates and updates the list of Runtimes
func (o *RuntimeOpt) Set(val string) error {
	k, v, ok := strings.Cut(val, "=")
	if !ok {
		return fmt.Errorf("invalid runtime argument: %s", val)
	}

	// TODO(thaJeztah): this should not accept spaces.
	k = strings.TrimSpace(k)
	v = strings.TrimSpace(v)
	if k == "" || v == "" {
		return fmt.Errorf("invalid runtime argument: %s", val)
	}

	// TODO(thaJeztah): this should not be case-insensitive.
	k = strings.ToLower(k)
	if k == o.stockRuntimeName {
		return fmt.Errorf("runtime name '%s' is reserved", o.stockRuntimeName)
	}

	if _, ok := (*o.values)[k]; ok {
		return fmt.Errorf("runtime '%s' was already defined", k)
	}

	(*o.values)[k] = types.Runtime{Path: v}

	return nil
}

// String returns Runtime values as a string.
func (o *RuntimeOpt) String() string {
	var out []string
	for k := range *o.values {
		out = append(out, k)
	}

	return fmt.Sprintf("%v", out)
}

// GetMap returns a map of Runtimes (name: path)
func (o *RuntimeOpt) GetMap() map[string]types.Runtime {
	if o.values != nil {
		return *o.values
	}

	return map[string]types.Runtime{}
}

// Type returns the type of the option
func (o *RuntimeOpt) Type() string {
	return "runtime"
}

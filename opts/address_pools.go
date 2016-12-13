package opts

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	types "github.com/docker/libnetwork/ipamutils"
)

// PoolsOpt is a Value type for parsing the default address pools definitions
type PoolsOpt struct {
	values *[]*types.PredefinedPools
}

// NewPoolsOpt creates a new PoolsOpt
func NewPoolsOpt(ref *[]*types.PredefinedPools) *PoolsOpt {
	return &PoolsOpt{values: ref}
}

// Set predefined pools
func (p *PoolsOpt) Set(value string) error {
	csvReader := csv.NewReader(strings.NewReader(value))
	fields, err := csvReader.Read()
	if err != nil {
		return err
	}

	poolsDef := types.PredefinedPools{}

	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("ninvalid field '%s' must be a key=value pair", field)
		}

		key := strings.ToLower(parts[0])
		value := strings.ToLower(parts[1])

		switch key {
		case "scope":
			poolsDef.Scope = value
		case "base":
			poolsDef.Base = value
		case "size":
			size, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid size value: %q (must be integer): %v", value, err)
			}
			poolsDef.Size = size
		default:
			return fmt.Errorf("unexpected key '%s' in '%s'", key, field)
		}
	}

	*p.values = append(*p.values, &poolsDef)

	return nil
}

// Type returns the type of this option
func (p *PoolsOpt) Type() string {
	return "default-address-pools"
}

// String returns a string repr of this option
func (p *PoolsOpt) String() string {
	pools := []string{}
	for _, pool := range *p.values {
		repr := fmt.Sprintf("%s %s %s", pool.Scope, pool.Base, pool.Size)
		pools = append(pools, repr)
	}
	return strings.Join(pools, ", ")
}

// Value returns the mounts
func (p *PoolsOpt) Value() []*types.PredefinedPools {
	var pd []*types.PredefinedPools
	for _, p := range *p.values {
		pd = append(pd, p)
	}
	return pd
}

package opts

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"

	swarmtypes "github.com/docker/docker/api/types/swarm"
)

// ConfigOpt is a Value type for parsing configs
type ConfigOpt struct {
	values []*swarmtypes.ConfigReference
}

// Set a new config value
func (o *ConfigOpt) Set(value string) error {
	csvReader := csv.NewReader(strings.NewReader(value))
	fields, err := csvReader.Read()
	if err != nil {
		return err
	}

	options := &swarmtypes.ConfigReference{
		File: &swarmtypes.ConfigReferenceFileTarget{
			UID:  "0",
			GID:  "0",
			Mode: 0444,
		},
	}

	// support a simple syntax of --config foo
	if len(fields) == 1 {
		options.File.Name = fields[0]
		options.ConfigName = fields[0]
		o.values = append(o.values, options)
		return nil
	}

	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		key := strings.ToLower(parts[0])

		if len(parts) != 2 {
			return fmt.Errorf("invalid field '%s' must be a key=value pair", field)
		}

		value := parts[1]
		switch key {
		case "source", "src":
			options.ConfigName = value
		case "target":
			options.File.Name = value
		case "uid":
			options.File.UID = value
		case "gid":
			options.File.GID = value
		case "mode":
			m, err := strconv.ParseUint(value, 0, 32)
			if err != nil {
				return fmt.Errorf("invalid mode specified: %v", err)
			}

			options.File.Mode = os.FileMode(m)
		default:
			return fmt.Errorf("invalid field in config request: %s", key)
		}
	}

	if options.ConfigName == "" {
		return fmt.Errorf("source is required")
	}

	o.values = append(o.values, options)
	return nil
}

// Type returns the type of this option
func (o *ConfigOpt) Type() string {
	return "config"
}

// String returns a string repr of this option
func (o *ConfigOpt) String() string {
	configs := []string{}
	for _, config := range o.values {
		repr := fmt.Sprintf("%s -> %s", config.ConfigName, config.File.Name)
		configs = append(configs, repr)
	}
	return strings.Join(configs, ", ")
}

// Value returns the config requests
func (o *ConfigOpt) Value() []*swarmtypes.ConfigReference {
	return o.values
}

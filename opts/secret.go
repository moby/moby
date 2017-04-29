package opts

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	swarmtypes "github.com/docker/docker/api/types/swarm"
)

// SecretOpt is a Value type for parsing secrets
type SecretOpt struct {
	values []*swarmtypes.SecretReference
}

// Set a new secret value
func (o *SecretOpt) Set(value string) error {
	csvReader := csv.NewReader(strings.NewReader(value))
	fields, err := csvReader.Read()
	if err != nil {
		return err
	}

	options := &swarmtypes.SecretReference{
		File: &swarmtypes.SecretReferenceFileTarget{
			UID:  "0",
			GID:  "0",
			Mode: 0444,
		},
	}

	// support a simple syntax of --secret foo
	if len(fields) == 1 {
		options.File.Name = fields[0]
		options.SecretName = fields[0]
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
			options.SecretName = value
		case "target":
			tDir, _ := filepath.Split(value)
			if tDir != "" {
				return fmt.Errorf("target must not be a path")
			}
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
			return fmt.Errorf("invalid field in secret request: %s", key)
		}
	}

	if options.SecretName == "" {
		return fmt.Errorf("source is required")
	}

	o.values = append(o.values, options)
	return nil
}

// Type returns the type of this option
func (o *SecretOpt) Type() string {
	return "secret"
}

// String returns a string repr of this option
func (o *SecretOpt) String() string {
	secrets := []string{}
	for _, secret := range o.values {
		repr := fmt.Sprintf("%s -> %s", secret.SecretName, secret.File.Name)
		secrets = append(secrets, repr)
	}
	return strings.Join(secrets, ", ")
}

// Value returns the secret requests
func (o *SecretOpt) Value() []*swarmtypes.SecretReference {
	return o.values
}

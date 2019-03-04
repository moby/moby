package opts

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
)

// ReadKVStrings reads a file of line terminated key=value pairs, and overrides any keys
// present in the file with additional pairs specified in the override parameter
func ReadKVStrings(files []string, override []string) ([]string, error) {
	return readKVStrings(files, override, nil)
}

// ReadKVEnvStrings reads a file of line terminated key=value pairs, and overrides any keys
// present in the file with additional pairs specified in the override parameter.
// If a key has no value, it will get the value from the environment.
func ReadKVEnvStrings(files []string, override []string) ([]string, error) {
	return readKVStrings(files, override, os.LookupEnv)
}

func readKVStrings(files []string, override []string, emptyFn func(string) (string, bool)) ([]string, error) {
	variables := []string{}
	for _, ef := range files {
		parsedVars, err := parseKeyValueFile(ef, emptyFn)
		if err != nil {
			return nil, err
		}
		variables = append(variables, parsedVars...)
	}
	// parse the '-e' and '--env' after, to allow override
	variables = append(variables, override...)

	return variables, nil
}

// ConvertKVStringsToMap converts ["key=value"] to {"key":"value"}
func ConvertKVStringsToMap(values []string) map[string]string {
	result := make(map[string]string, len(values))
	for _, value := range values {
		kv := strings.SplitN(value, "=", 2)
		if len(kv) == 1 {
			result[kv[0]] = ""
		} else {
			result[kv[0]] = kv[1]
		}
	}

	return result
}

// ConvertKVStringsToMapWithNil converts ["key=value"] to {"key":"value"}
// but set unset keys to nil - meaning the ones with no "=" in them.
// We use this in cases where we need to distinguish between
//   FOO=  and FOO
// where the latter case just means FOO was mentioned but not given a value
func ConvertKVStringsToMapWithNil(values []string) map[string]*string {
	result := make(map[string]*string, len(values))
	for _, value := range values {
		kv := strings.SplitN(value, "=", 2)
		if len(kv) == 1 {
			result[kv[0]] = nil
		} else {
			result[kv[0]] = &kv[1]
		}
	}

	return result
}

// ParseRestartPolicy returns the parsed policy or an error indicating what is incorrect
func ParseRestartPolicy(policy string) (container.RestartPolicy, error) {
	p := container.RestartPolicy{}

	if policy == "" {
		return p, nil
	}

	parts := strings.Split(policy, ":")

	if len(parts) > 2 {
		return p, fmt.Errorf("invalid restart policy format")
	}
	if len(parts) == 2 {
		count, err := strconv.Atoi(parts[1])
		if err != nil {
			return p, fmt.Errorf("maximum retry count must be an integer")
		}

		p.MaximumRetryCount = count
	}

	p.Name = parts[0]

	return p, nil
}

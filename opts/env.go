package opts // import "github.com/docker/docker/opts"

import (
	"os"
	"strings"

	"github.com/pkg/errors"
)

// ValidateEnv validates an environment variable and returns it.
// If no value is specified, it obtains its value from the current environment
//
// As on ParseEnvFile and related to #16585, environment variable names
// are not validate whatsoever, it's up to application inside docker
// to validate them or not.
//
// The only validation here is to check if name is empty, per #25099
func ValidateEnv(val string) (string, error) {
	arr := strings.SplitN(val, "=", 2)
	if arr[0] == "" {
		return "", errors.New("invalid environment variable: " + val)
	}
	if len(arr) > 1 {
		return val, nil
	}
	if envVal, ok := os.LookupEnv(arr[0]); ok {
		return arr[0] + "=" + envVal, nil
	}
	return val, nil
}

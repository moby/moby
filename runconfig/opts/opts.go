package opts

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"

	fopts "github.com/docker/docker/opts"
)

// ValidateAttach validates that the specified string is a valid attach option.
func ValidateAttach(val string) (string, error) {
	s := strings.ToLower(val)
	for _, str := range []string{"stdin", "stdout", "stderr"} {
		if s == str {
			return s, nil
		}
	}
	return val, fmt.Errorf("valid streams are STDIN, STDOUT and STDERR")
}

// ValidateEnv validates an environment variable and returns it.
// If no value is specified, it returns the current value using os.Getenv.
//
// As on ParseEnvFile and related to #16585, environment variable names
// are not validate what so ever, it's up to application inside docker
// to validate them or not.
//
// The only validation here is to check if name is empty, per #25099
func ValidateEnv(val string) (string, error) {
	arr := strings.Split(val, "=")
	if arr[0] == "" {
		return "", fmt.Errorf("invalid environment variable: %s", val)
	}
	if len(arr) > 1 {
		return val, nil
	}
	if !doesEnvExist(val) {
		return val, nil
	}
	return fmt.Sprintf("%s=%s", val, os.Getenv(val)), nil
}

func doesEnvExist(name string) bool {
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if runtime.GOOS == "windows" {
			// Environment variable are case-insensitive on Windows. PaTh, path and PATH are equivalent.
			if strings.EqualFold(parts[0], name) {
				return true
			}
		}
		if parts[0] == name {
			return true
		}
	}
	return false
}

// ValidateExtraHost validates that the specified string is a valid extrahost and returns it.
// ExtraHost is in the form of name:ip where the ip has to be a valid ip (IPv4 or IPv6).
func ValidateExtraHost(val string) (string, error) {
	// allow for IPv6 addresses in extra hosts by only splitting on first ":"
	arr := strings.SplitN(val, ":", 2)
	if len(arr) != 2 || len(arr[0]) == 0 {
		return "", fmt.Errorf("bad format for add-host: %q", val)
	}
	if _, err := fopts.ValidateIPAddress(arr[1]); err != nil {
		return "", fmt.Errorf("invalid IP address in add-host: %q", arr[1])
	}
	return val, nil
}

// ValidateMACAddress validates a MAC address.
func ValidateMACAddress(val string) (string, error) {
	_, err := net.ParseMAC(strings.TrimSpace(val))
	if err != nil {
		return "", err
	}
	return val, nil
}

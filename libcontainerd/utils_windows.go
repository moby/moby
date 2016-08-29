package libcontainerd

import (
	"strconv"
	"strings"
)

// setupEnvironmentVariables converts a string array of environment variables
// into a map as required by the HCS. Source array is in format [v1=k1] [v2=k2] etc.
func setupEnvironmentVariables(a []string) map[string]string {
	r := make(map[string]string)
	for _, s := range a {
		arr := strings.Split(s, "=")
		if len(arr) == 2 {
			r[arr[0]] = arr[1]
		}
	}
	return r
}

// Apply for a servicing option is a no-op.
func (s *ServicingOption) Apply(interface{}) error {
	return nil
}

// buildFromVersion takes an image version string and returns the Windows build
// number. It returns 0 if the build number is not present.
func buildFromVersion(osver string) int {
	v := strings.Split(osver, ".")
	if len(v) < 3 {
		return 0
	}
	if build, err := strconv.Atoi(v[2]); err == nil {
		return build
	}
	return 0
}

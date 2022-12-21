package local // import "github.com/docker/docker/libcontainerd/local"

import "strings"

// setupEnvironmentVariables converts a string array of environment variables
// into a map as required by the HCS. Source array is in format [v1=k1] [v2=k2] etc.
func setupEnvironmentVariables(a []string) map[string]string {
	r := make(map[string]string)
	for _, s := range a {
		if k, v, ok := strings.Cut(s, "="); ok {
			r[k] = v
		}
	}
	return r
}

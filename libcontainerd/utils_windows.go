package libcontainerd

import "strings"

// setupEnvironmentVariables convert a string array of environment variables
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

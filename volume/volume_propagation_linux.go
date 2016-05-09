// +build linux

package volume

import (
	"strings"
)

// DefaultPropagationMode defines what propagation mode should be used by
// default if user has not specified one explicitly.
const DefaultPropagationMode string = "rprivate"

// propagation modes
var propagationModes = map[string]bool{
	"private":  true,
	"rprivate": true,
	"slave":    true,
	"rslave":   true,
	"shared":   true,
	"rshared":  true,
}

// GetPropagation extracts and returns the mount propagation mode. If there
// are no specifications, then by default it is "private".
func GetPropagation(mode string) string {
	for _, o := range strings.Split(mode, ",") {
		if propagationModes[o] {
			return o
		}
	}
	return DefaultPropagationMode
}

// HasPropagation checks if there is a valid propagation mode present in
// passed string. Returns true if a valid propagation mode specifier is
// present, false otherwise.
func HasPropagation(mode string) bool {
	for _, o := range strings.Split(mode, ",") {
		if propagationModes[o] {
			return true
		}
	}
	return false
}

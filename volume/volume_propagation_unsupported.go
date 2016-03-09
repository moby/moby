// +build !linux

package volume

// DefaultPropagationMode is used only in linux. In other cases it returns
// empty string.
const DefaultPropagationMode string = ""

// propagation modes not supported on this platform.
var propagationModes = map[string]bool{}

// GetPropagation is not supported. Return empty string.
func GetPropagation(mode string) string {
	return DefaultPropagationMode
}

// HasPropagation checks if there is a valid propagation mode present in
// passed string. Returns true if a valid propagation mode specifier is
// present, false otherwise.
func HasPropagation(mode string) bool {
	return false
}

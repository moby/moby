package mounts // import "github.com/moby/moby/volume/mounts"

import "strings"

// {<copy mode>=isEnabled}
var copyModes = map[string]bool{
	"nocopy": false,
}

func copyModeExists(mode string) bool {
	_, exists := copyModes[mode]
	return exists
}

// GetCopyMode gets the copy mode from the mode string for mounts
func getCopyMode(mode string, def bool) (bool, bool) {
	for _, o := range strings.Split(mode, ",") {
		if isEnabled, exists := copyModes[o]; exists {
			return isEnabled, true
		}
	}
	return def, false
}

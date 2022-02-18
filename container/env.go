package container // import "github.com/moby/moby/container"

import (
	"strings"
)

// ReplaceOrAppendEnvValues returns the defaults with the overrides either
// replaced by env key or appended to the list
func ReplaceOrAppendEnvValues(defaults, overrides []string) []string {
	cache := make(map[string]int, len(defaults))
	for i, e := range defaults {
		index := strings.Index(e, "=")
		cache[e[:index]] = i
	}

	for _, value := range overrides {
		// Values w/o = means they want this env to be removed/unset.
		index := strings.Index(value, "=")
		if index < 0 {
			// no "=" in value
			if i, exists := cache[value]; exists {
				defaults[i] = "" // Used to indicate it should be removed
			}
			continue
		}

		if i, exists := cache[value[:index]]; exists {
			defaults[i] = value
		} else {
			defaults = append(defaults, value)
		}
	}

	// Now remove all entries that we want to "unset"
	for i := 0; i < len(defaults); i++ {
		if defaults[i] == "" {
			defaults = append(defaults[:i], defaults[i+1:]...)
			i--
		}
	}

	return defaults
}

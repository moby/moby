package caps

import (
	"fmt"
	"slices"
	"strings"

	"github.com/moby/moby/v2/errdefs"
)

var (
	allCaps []string

	// knownCapabilities is a map of all known capabilities, using capability
	// name as index. Nil values indicate that the capability is known, but either
	// not supported by the Kernel, or not available in the current environment,
	// for example, when running Docker-in-Docker with restricted capabilities.
	//
	// Capabilities are one of the security systems in Linux Security Module (LSM)
	// framework provided by the kernel.
	// For more details on capabilities, see http://man7.org/linux/man-pages/man7/capabilities.7.html
	knownCaps map[string]*struct{}
)

// GetAllCapabilities returns all capabilities that are available in the current
// environment.
func GetAllCapabilities() []string {
	initCaps()
	return allCaps
}

// knownCapabilities returns a map of all known capabilities, using capability
// name as index. Nil values indicate that the capability is known, but either
// not supported by the Kernel, or not available in the current environment, for
// example, when running Docker-in-Docker with restricted capabilities.
func knownCapabilities() map[string]*struct{} {
	initCaps()
	return knownCaps
}

const allCapabilities = "ALL"

// NormalizeLegacyCapabilities normalizes, and validates CapAdd/CapDrop capabilities
// by upper-casing them, and adding a CAP_ prefix (if not yet present).
//
// This function also accepts the "ALL" magic-value, that's used by CapAdd/CapDrop.
func NormalizeLegacyCapabilities(caps []string) ([]string, error) {
	var (
		normalized     []string
		capabilityList = knownCapabilities()
	)

	for _, c := range caps {
		c = strings.ToUpper(c)
		if c == allCapabilities {
			normalized = append(normalized, c)
			continue
		}
		if !strings.HasPrefix(c, "CAP_") {
			c = "CAP_" + c
		}
		if v, ok := capabilityList[c]; !ok {
			return nil, errdefs.InvalidParameter(fmt.Errorf("unknown capability: %q", c))
		} else if v == nil {
			return nil, errdefs.InvalidParameter(fmt.Errorf("capability not supported by your kernel or not available in the current environment: %q", c))
		}
		normalized = append(normalized, c)
	}
	return normalized, nil
}

// TweakCapabilities tweaks capabilities by adding, dropping, or overriding
// capabilities in the basics capabilities list. All capabilities are added
// if privileged is true.
func TweakCapabilities(basics, adds, drops []string, privileged bool) ([]string, error) {
	switch {
	case privileged:
		// Privileged containers get all capabilities
		return GetAllCapabilities(), nil
	case len(adds) == 0 && len(drops) == 0:
		// Nothing to tweak; we're done
		return basics, nil
	}

	capDrop, err := NormalizeLegacyCapabilities(drops)
	if err != nil {
		return nil, err
	}
	capAdd, err := NormalizeLegacyCapabilities(adds)
	if err != nil {
		return nil, err
	}

	var caps []string

	switch {
	case slices.Contains(capAdd, allCapabilities):
		// Add all capabilities except ones on capDrop
		for _, c := range GetAllCapabilities() {
			if !slices.Contains(capDrop, c) {
				caps = append(caps, c)
			}
		}
	case slices.Contains(capDrop, allCapabilities):
		// "Drop" all capabilities; use what's in capAdd instead
		caps = capAdd
	default:
		// First drop some capabilities
		for _, c := range basics {
			if !slices.Contains(capDrop, c) {
				caps = append(caps, c)
			}
		}
		// Then add the list of capabilities from capAdd
		caps = append(caps, capAdd...)
	}
	return caps, nil
}

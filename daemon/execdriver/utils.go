package execdriver

import (
	"fmt"
	"strings"

	"github.com/docker/docker/utils"
	"github.com/docker/libcontainer/security/capabilities"
)

func TweakCapabilities(basics, adds, drops []string) ([]string, error) {
	var (
		newCaps []string
		allCaps = capabilities.GetAllCapabilities()
	)

	// look for invalid cap in the drop list
	for _, cap := range drops {
		if strings.ToLower(cap) == "all" {
			continue
		}
		if !utils.StringsContainsNoCase(allCaps, cap) {
			return nil, fmt.Errorf("Unknown capability drop: %q", cap)
		}
	}

	// handle --cap-add=all
	if utils.StringsContainsNoCase(adds, "all") {
		basics = capabilities.GetAllCapabilities()
	}

	if !utils.StringsContainsNoCase(drops, "all") {
		for _, cap := range basics {
			// skip `all` aready handled above
			if strings.ToLower(cap) == "all" {
				continue
			}

			// if we don't drop `all`, add back all the non-dropped caps
			if !utils.StringsContainsNoCase(drops, cap) {
				newCaps = append(newCaps, strings.ToUpper(cap))
			}
		}
	}

	for _, cap := range adds {
		// skip `all` aready handled above
		if strings.ToLower(cap) == "all" {
			continue
		}

		if !utils.StringsContainsNoCase(allCaps, cap) {
			return nil, fmt.Errorf("Unknown capability to add: %q", cap)
		}

		// add cap if not already in the list
		if !utils.StringsContainsNoCase(newCaps, cap) {
			newCaps = append(newCaps, strings.ToUpper(cap))
		}
	}

	return newCaps, nil
}

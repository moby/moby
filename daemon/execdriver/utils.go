package execdriver

import (
	"strings"

	"github.com/docker/libcontainer/security/capabilities"
	"github.com/dotcloud/docker/utils"
)

func TweakCapabilities(basics, adds, drops []string) []string {
	var caps []string

	if utils.StringsContainsNoCase(adds, "all") {
		basics = capabilities.GetAllCapabilities()
	}

	if !utils.StringsContainsNoCase(drops, "all") {
		for _, cap := range basics {
			if !utils.StringsContainsNoCase(drops, cap) {
				caps = append(caps, cap)
			}
		}
	}

	for _, cap := range adds {
		if strings.ToLower(cap) == "all" {
			continue
		}
		if !utils.StringsContainsNoCase(caps, cap) {
			caps = append(caps, cap)
		}
	}
	return caps
}

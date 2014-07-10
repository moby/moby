package execdriver

import "github.com/dotcloud/docker/utils"

func TweakCapabilities(basics, adds, drops []string) []string {
	var caps []string
	for _, cap := range basics {
		if !utils.StringsContains(drops, cap) {
			caps = append(caps, cap)
		}
	}

	for _, cap := range adds {
		if !utils.StringsContains(caps, cap) {
			caps = append(caps, cap)
		}
	}
	return caps
}

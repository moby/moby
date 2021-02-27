package oci // import "github.com/docker/docker/oci"

import (
	"fmt"
	"regexp"
	"strconv"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// nolint: gosimple
var deviceCgroupRuleRegex = regexp.MustCompile("^([acb]) ([0-9]+|\\*):([0-9]+|\\*) ([rwm]{1,3})$|a$")

// This regex checks if a cgroup rule addressed to an 'a' device type is effective, given that 'a' maps to 'a *:* rwm'
// If the rule is just 'a' or 'a *:* rwm', it is deemed correct, and if not, like 'a 1:3 m', we can let the user know that the rule is ineffective
// nolint: gosimple
var deviceCgroupARuleRegex = regexp.MustCompile("^a \\*:\\* ([rwm]{3})$|a$")

// SetCapabilities sets the provided capabilities on the spec
// All capabilities are added if privileged is true
func SetCapabilities(s *specs.Spec, caplist []string) error {
	s.Process.Capabilities.Effective = caplist
	s.Process.Capabilities.Bounding = caplist
	s.Process.Capabilities.Permitted = caplist
	s.Process.Capabilities.Inheritable = caplist
	// setUser has already been executed here
	// if non root drop capabilities in the way execve does
	if s.Process.User.UID != 0 {
		s.Process.Capabilities.Effective = []string{}
		s.Process.Capabilities.Permitted = []string{}
	}
	return nil
}

// DevicePermissionsFromCgroupRules takes rules for the devices cgroup
func DevicePermissionsFromCgroupRules(rules []string) ([]specs.LinuxDeviceCgroup, []string, error) {
	var devPermissions []specs.LinuxDeviceCgroup
	var warnings []string
	for _, deviceCgroupRule := range rules {
		ss := deviceCgroupRuleRegex.FindAllStringSubmatch(deviceCgroupRule, -1)
		if len(ss) == 0 || len(ss[0]) != 5 {
			return nil, warnings, fmt.Errorf("invalid device cgroup rule format: '%s'", deviceCgroupRule)
		}
		matches := ss[0]

		major := int64(-1)
		minor := int64(-1)

		if matches[0] == "a" || matches[1] == "a" {
			ms := deviceCgroupARuleRegex.MatchString(matches[0])
			if !ms {
				warnings = append(warnings, fmt.Sprintf("although this cgroup rule is technically correct, because 'a' maps to 'a *:* rwm' regardless of what comes next, this format is partially ineffective: '%s'", deviceCgroupRule))
			}

			dPermissions := specs.LinuxDeviceCgroup{
				Allow:  true,
				Type:   "a",
				Major:  &major,
				Minor:  &minor,
				Access: "rwm",
			}

			devPermissions = append(devPermissions, dPermissions)
			return devPermissions, warnings, nil
		}

		dPermissions := specs.LinuxDeviceCgroup{
			Allow:  true,
			Type:   matches[1],
			Major:  &major,
			Minor:  &minor,
			Access: matches[4],
		}
		if matches[2] != "*" && matches[2] != "-1" {
			m, err := strconv.ParseUint(matches[2], 10, 12)
			if err != nil {
				return nil, warnings, fmt.Errorf("major value out of range in device cgroup rule format: '%s'", deviceCgroupRule)
			}
			major := int64(m)
			dPermissions.Major = &major
		}
		if matches[3] != "*" && matches[2] != "-1" {
			m, err := strconv.ParseUint(matches[3], 10, 20)
			if err != nil {
				return nil, warnings, fmt.Errorf("minor value out of range in device cgroup rule format: '%s'", deviceCgroupRule)
			}
			minor := int64(m)
			dPermissions.Minor = &minor
		}
		devPermissions = append(devPermissions, dPermissions)
	}
	return devPermissions, warnings, nil
}

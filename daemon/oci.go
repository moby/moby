package daemon // import "github.com/docker/docker/daemon"

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/caps"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// nolint: gosimple
var (
	deviceCgroupRuleRegex = regexp.MustCompile("^([acb]) ([0-9]+|\\*):([0-9]+|\\*) ([rwm]{1,3})$")
)

func setCapabilities(s *specs.Spec, c *container.Container) error {
	var caplist []string
	var err error
	if c.HostConfig.Privileged {
		caplist = caps.GetAllCapabilities()
	} else {
		caplist, err = caps.TweakCapabilities(s.Process.Capabilities.Bounding, c.HostConfig.CapAdd, c.HostConfig.CapDrop)
		if err != nil {
			return err
		}
	}
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

func appendDevicePermissionsFromCgroupRules(devPermissions []specs.LinuxDeviceCgroup, rules []string) ([]specs.LinuxDeviceCgroup, error) {
	for _, deviceCgroupRule := range rules {
		ss := deviceCgroupRuleRegex.FindAllStringSubmatch(deviceCgroupRule, -1)
		if len(ss[0]) != 5 {
			return nil, fmt.Errorf("invalid device cgroup rule format: '%s'", deviceCgroupRule)
		}
		matches := ss[0]

		dPermissions := specs.LinuxDeviceCgroup{
			Allow:  true,
			Type:   matches[1],
			Access: matches[4],
		}
		if matches[2] == "*" {
			major := int64(-1)
			dPermissions.Major = &major
		} else {
			major, err := strconv.ParseInt(matches[2], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid major value in device cgroup rule format: '%s'", deviceCgroupRule)
			}
			dPermissions.Major = &major
		}
		if matches[3] == "*" {
			minor := int64(-1)
			dPermissions.Minor = &minor
		} else {
			minor, err := strconv.ParseInt(matches[3], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid minor value in device cgroup rule format: '%s'", deviceCgroupRule)
			}
			dPermissions.Minor = &minor
		}
		devPermissions = append(devPermissions, dPermissions)
	}
	return devPermissions, nil
}

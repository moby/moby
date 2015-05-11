// +build windows

package runconfig

import (
	flag "github.com/docker/docker/pkg/mflag"
)

// There are no platform specific flags to parse currently on Windows
func parsePlatformSpecific(cmd *flag.FlagSet, hostconfig *HostConfig, netMode NetworkMode) error {
	return nil
}

// +build !windows,!linux

package runconfig

import (
	flag "github.com/docker/docker/pkg/mflag"
)

func parsePlatformSpecific(cmd *flag.FlagSet, hostconfig *HostConfig, netMode NetworkMode) error {
	return nil
}

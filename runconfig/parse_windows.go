package runconfig

import (
	"strings"

	derr "github.com/docker/docker/errors"
)

// ValidateNetMode ensures that the various combinations of requested
// network settings are valid.
func ValidateNetMode(c *Config, hc *HostConfig) error {
	// We may not be passed a host config, such as in the case of docker commit
	if hc == nil {
		return nil
	}
	parts := strings.Split(string(hc.NetworkMode), ":")
	switch mode := parts[0]; mode {
	case "default", "none":
	default:
		return derr.ErrorCodeInvalidNetworkOption.WithArgs(hc.NetworkMode)
	}
	return nil
}

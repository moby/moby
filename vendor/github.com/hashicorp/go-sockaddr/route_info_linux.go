package sockaddr

import (
	"errors"
	"os/exec"
)

var cmds map[string][]string = map[string][]string{
	"ip": {"/sbin/ip", "route"},
}

type routeInfo struct {
	cmds map[string][]string
}

// NewRouteInfo returns a Linux-specific implementation of the RouteInfo
// interface.
func NewRouteInfo() (routeInfo, error) {
	return routeInfo{
		cmds: cmds,
	}, nil
}

// GetDefaultInterfaceName returns the interface name attached to the default
// route on the default interface.
func (ri routeInfo) GetDefaultInterfaceName() (string, error) {
	out, err := exec.Command(cmds["ip"][0], cmds["ip"][1:]...).Output()
	if err != nil {
		return "", err
	}

	var ifName string
	if ifName, err = parseDefaultIfNameFromIPCmd(string(out)); err != nil {
		return "", errors.New("No default interface found")
	}
	return ifName, nil
}

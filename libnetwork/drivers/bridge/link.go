//go:build linux

package bridge

import (
	"context"
	"fmt"
	"net"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/types"
)

type link struct {
	parentIP string
	childIP  string
	ports    []types.TransportPort
	bridge   string
}

func (l *link) String() string {
	return fmt.Sprintf("%s <-> %s [%v] on %s", l.parentIP, l.childIP, l.ports, l.bridge)
}

func newLink(parentIP, childIP string, ports []types.TransportPort, bridge string) *link {
	return &link{
		childIP:  childIP,
		parentIP: parentIP,
		ports:    ports,
		bridge:   bridge,
	}
}

func (l *link) Enable() error {
	linkFunction := func() error {
		return linkContainers(iptables.Append, l.parentIP, l.childIP, l.ports, l.bridge, false)
	}

	iptables.OnReloaded(func() { linkFunction() })
	return linkFunction()
}

func (l *link) Disable() {
	if err := linkContainers(iptables.Delete, l.parentIP, l.childIP, l.ports, l.bridge, true); err != nil {
		// @TODO: Return error once we have the iptables package return typed errors.
		log.G(context.TODO()).WithError(err).Errorf("Error removing IPTables rules for link: %s", l.String())
	}
}

func linkContainers(action iptables.Action, parentIP, childIP string, ports []types.TransportPort, bridge string, ignoreErrors bool) error {
	ip1 := net.ParseIP(parentIP)
	if ip1 == nil {
		return fmt.Errorf("cannot link to a container with an invalid parent IP address %q", parentIP)
	}
	ip2 := net.ParseIP(childIP)
	if ip2 == nil {
		return fmt.Errorf("cannot link to a container with an invalid child IP address %q", childIP)
	}

	chain := iptables.ChainInfo{Name: DockerChain}
	for _, port := range ports {
		err := chain.Link(action, ip1, ip2, int(port.Port), port.Proto.String(), bridge)
		if !ignoreErrors && err != nil {
			return err
		}
	}
	return nil
}

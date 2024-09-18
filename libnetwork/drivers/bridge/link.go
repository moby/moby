//go:build linux

package bridge

import (
	"context"
	"fmt"
	"net"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/types"
)

type link struct {
	parentIP net.IP
	childIP  net.IP
	ports    []types.TransportPort
	bridge   string
}

func (l *link) String() string {
	return fmt.Sprintf("%s <-> %s [%v] on %s", l.parentIP, l.childIP, l.ports, l.bridge)
}

func newLink(parentIP, childIP net.IP, ports []types.TransportPort, bridge string) (*link, error) {
	if parentIP == nil {
		return nil, fmt.Errorf("cannot link to a container with an empty parent IP address")
	}
	if childIP == nil {
		return nil, fmt.Errorf("cannot link to a container with an empty child IP address")
	}

	return &link{
		childIP:  childIP,
		parentIP: parentIP,
		ports:    ports,
		bridge:   bridge,
	}, nil
}

func (l *link) Enable() error {
	linkFunction := func() error {
		return linkContainers(iptables.Append, l.parentIP, l.childIP, l.ports, l.bridge, false)
	}
	if err := linkFunction(); err != nil {
		return err
	}

	iptables.OnReloaded(func() { _ = linkFunction() })
	return nil
}

func (l *link) Disable() {
	if err := linkContainers(iptables.Delete, l.parentIP, l.childIP, l.ports, l.bridge, true); err != nil {
		// @TODO: Return error once we have the iptables package return typed errors.
		log.G(context.TODO()).WithError(err).Errorf("Error removing IPTables rules for link: %s", l.String())
	}
}

func linkContainers(action iptables.Action, parentIP, childIP net.IP, ports []types.TransportPort, bridge string, ignoreErrors bool) error {
	if parentIP == nil {
		return fmt.Errorf("cannot link to a container with an empty parent IP address")
	}
	if childIP == nil {
		return fmt.Errorf("cannot link to a container with an empty child IP address")
	}

	chain := iptables.ChainInfo{Name: DockerChain}
	for _, port := range ports {
		err := chain.Link(action, parentIP, childIP, int(port.Port), port.Proto.String(), bridge)
		if !ignoreErrors && err != nil {
			return err
		}
	}
	return nil
}

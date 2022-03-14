//go:build linux
// +build linux

package bridge

import (
	"fmt"
	"net"

	"github.com/docker/docker/libnetwork/firewallapi"
	"github.com/docker/docker/libnetwork/firewalld"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/nftables"
	"github.com/docker/docker/libnetwork/types"
	"github.com/sirupsen/logrus"
)

type link struct {
	parentIP       string
	childIP        string
	ports          []types.TransportPort
	bridge         string
	enableNFTables bool
}

func (l *link) String() string {
	return fmt.Sprintf("%s <-> %s [%v] on %s", l.parentIP, l.childIP, l.ports, l.bridge)
}

func newLink(parentIP, childIP string, ports []types.TransportPort, bridge string, enableNFTables bool) *link {
	return &link{
		childIP:        childIP,
		parentIP:       parentIP,
		ports:          ports,
		bridge:         bridge,
		enableNFTables: enableNFTables,
	}

}

func (l *link) Enable() error {
	// -A == iptables append flag
	linkFunction := func() error {
		return linkContainers("-A", l.parentIP, l.childIP, l.ports, l.bridge, l.enableNFTables, false)
	}

	firewalld.OnReloaded(func() { linkFunction() })
	return linkFunction()
}

func (l *link) Disable() {
	// -D == iptables delete flag
	err := linkContainers("-D", l.parentIP, l.childIP, l.ports, l.bridge, l.enableNFTables, true)
	if err != nil {
		logrus.Errorf("Error removing rules for a link %s due to %s", l.String(), err.Error())
	}
	// Return proper error once we move to use a proper iptables package
	// that returns typed errors
}

func linkContainers(action, parentIP, childIP string, ports []types.TransportPort, bridge string,
	enableNFTables bool, ignoreErrors bool) error {
	var nfAction firewallapi.Action
	var chain firewallapi.FirewallChain

	if enableNFTables {
		chain = nftables.ChainInfo{Name: DockerChain}
		switch action {
		case "-A":
			nfAction = nftables.Append
		case "-I":
			nfAction = nftables.Insert
		case "-D":
			nfAction = nftables.Delete
		default:
			return InvalidNFTablesCfgError(action)
		}
	} else {
		chain = iptables.ChainInfo{Name: DockerChain}
		switch action {
		case "-A":
			nfAction = iptables.Append
		case "-I":
			nfAction = iptables.Insert
		case "-D":
			nfAction = iptables.Delete
		default:
			return InvalidIPTablesCfgError(action)
		}
	}

	ip1 := net.ParseIP(parentIP)
	if ip1 == nil {
		return InvalidLinkIPAddrError(parentIP)
	}
	ip2 := net.ParseIP(childIP)
	if ip2 == nil {
		return InvalidLinkIPAddrError(childIP)
	}

	for _, port := range ports {
		err := chain.Link(nfAction, ip1, ip2, int(port.Port), port.Proto.String(), bridge)
		if !ignoreErrors && err != nil {
			return err
		}
	}
	return nil
}

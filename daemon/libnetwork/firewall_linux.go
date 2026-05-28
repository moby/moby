package libnetwork

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/nftables"
	"github.com/moby/moby/v2/daemon/libnetwork/iptables"
)

const userChain = "DOCKER-USER"

func (c *Controller) selectFirewallBackend() error {
	// If explicitly configured to use iptables, don't consider nftables.
	if c.cfg.FirewallBackend == "iptables" {
		return nil
	}
	// If configured to use nftables, but it can't be initialised, return an error.
	if c.cfg.FirewallBackend == "nftables" {
		if err := nftables.Enable(); err != nil {
			return fmt.Errorf("firewall-backend is set to nftables: %v", err)
		}
		return nil
	}
	return nil
}

func (c *Controller) setupPlatformFirewall() {
	c.setupUserChains()
	// Add handler for iptables rules restoration in case of a firewalld reload
	c.handleFirewalldReload()
}

// Sets up the DOCKER-USER chain for each iptables version (IPv4, IPv6) that's
// enabled in the controller's configuration.
func (c *Controller) setupUserChains() {
	// There's no equivalent to DOCKER-USER in the nftables implementation.
	if nftables.Enabled() {
		return
	}

	setup := func() error {
		var errs []error
		for _, ipVersion := range c.enabledIptablesVersions() {
			errs = append(errs, setupUserChain(ipVersion))
		}
		return errors.Join(errs...)
	}
	if err := setup(); err != nil {
		log.G(context.Background()).WithError(err).Warn("configuring " + userChain)
	}
	iptables.OnReloaded(func() {
		if err := setup(); err != nil {
			log.G(context.Background()).WithError(err).Warn("configuring " + userChain + " on firewall reload")
		}
	})
}

// setupUserChain sets up the DOCKER-USER chain for the given [iptables.IPVersion].
//
// This chain allows users to configure firewall policies in a way that
// persist daemon operations/restarts. The daemon does not delete or modify
// any pre-existing rules from the DOCKER-USER filter chain.
//
// Once the DOCKER-USER chain is created, the daemon does not remove it when
// IPTableForwarding is disabled, because it contains rules configured by user
// that are beyond the daemon's control.
func setupUserChain(ipVersion iptables.IPVersion) error {
	ipt := iptables.GetIptable(ipVersion)
	if _, err := ipt.NewChain(userChain, iptables.Filter); err != nil {
		return fmt.Errorf("failed to create %s %v chain: %v", userChain, ipVersion, err)
	}
	if err := ipt.EnsureJumpRule(iptables.Filter, "FORWARD", userChain); err != nil {
		return fmt.Errorf("failed to ensure the jump rule for %s %v: %w", userChain, ipVersion, err)
	}
	return nil
}

func (c *Controller) handleFirewalldReload() {
	handler := func() {
		services := make(map[serviceKey]*service)

		c.mu.Lock()
		for k, s := range c.serviceBindings {
			if k.ports != "" && len(s.ingressPorts) != 0 {
				services[k] = s
			}
		}
		c.mu.Unlock()

		for _, s := range services {
			c.handleFirewallReloadService(s)
		}
	}
	// Add handler for iptables rules restoration in case of a firewalld reload
	iptables.OnReloaded(handler)
}

func (c *Controller) handleFirewallReloadService(s *service) {
	s.Lock()
	defer s.Unlock()
	if s.deleted {
		log.G(context.TODO()).Debugf("handleFirewallReloadService called for deleted service %s/%s", s.id, s.name)
		return
	}
	for nid := range s.loadBalancers {
		n, err := c.NetworkByID(nid)
		if err != nil {
			continue
		}
		ep, sb, err := n.findLBEndpointSandbox()
		if err != nil {
			log.G(context.TODO()).Warnf("handleFirewallReloadService failed to find LB Endpoint Sandbox for %s/%s: %v -- ", n.ID(), n.Name(), err)
			continue
		}
		if sb.osSbox == nil {
			return
		}
		if ep != nil {
			var gwIP net.IP
			if gwEP, _ := sb.getGatewayEndpoint(); gwEP != nil {
				gwIP = gwEP.Iface().Address().IP
			}
			if err := restoreIngressPorts(gwIP, s.ingressPorts); err != nil {
				log.G(context.TODO()).Errorf("Failed to add ingress: %v", err)
				return
			}
		}
	}
}

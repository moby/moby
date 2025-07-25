package libnetwork

import (
	"context"
	"fmt"
	"net"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/iptables"
)

const userChain = "DOCKER-USER"

var ctrl *Controller

func setupArrangeUserFilterRule(c *Controller) {
	ctrl = c
	iptables.OnReloaded(arrangeUserFilterRule)
}

// arrangeUserFilterRule sets up the DOCKER-USER chain for each iptables version
// (IPv4, IPv6) that's enabled in the controller's configuration.
func arrangeUserFilterRule() {
	if ctrl == nil {
		return
	}
	for _, ipVersion := range ctrl.enabledIptablesVersions() {
		if err := setupUserChain(ipVersion); err != nil {
			log.G(context.TODO()).WithError(err).Warn("arrangeUserFilterRule")
		}
	}
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
	if _, err := ipt.NewChain(userChain, iptables.Filter, false); err != nil {
		return fmt.Errorf("failed to create %s %v chain: %v", userChain, ipVersion, err)
	}
	if err := ipt.AddReturnRule(iptables.Filter, userChain); err != nil {
		return fmt.Errorf("failed to add the RETURN rule for %s %v: %w", userChain, ipVersion, err)
	}
	if err := ipt.EnsureJumpRule(iptables.Filter, "FORWARD", userChain); err != nil {
		return fmt.Errorf("failed to ensure the jump rule for %s %v: %w", userChain, ipVersion, err)
	}
	return nil
}

func (c *Controller) setupPlatformFirewall() {
	setupArrangeUserFilterRule(c)

	// Add handler for iptables rules restoration in case of a firewalld reload
	c.handleFirewalldReload()
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
			if gwEP := sb.getGatewayEndpoint(); gwEP != nil {
				gwIP = gwEP.Iface().Address().IP
			}
			if err := restoreIngressPorts(gwIP, s.ingressPorts); err != nil {
				log.G(context.TODO()).Errorf("Failed to add ingress: %v", err)
				return
			}
		}
	}
}

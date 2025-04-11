package buildkit

import (
	"net"

	"github.com/docker/docker/daemon/config"
	"github.com/moby/buildkit/executor/oci"
)

func ipAddresses(ips []net.IP) []string {
	var addrs []string
	for _, ip := range ips {
		addrs = append(addrs, ip.String())
	}
	return addrs
}

func getDNSConfig(cfg config.DNSConfig) *oci.DNSConfig {
	if cfg.DNS != nil || cfg.DNSSearch != nil || cfg.DNSOptions != nil {
		return &oci.DNSConfig{
			Nameservers:   ipAddresses(cfg.DNS),
			SearchDomains: cfg.DNSSearch,
			Options:       cfg.DNSOptions,
		}
	}
	return nil
}

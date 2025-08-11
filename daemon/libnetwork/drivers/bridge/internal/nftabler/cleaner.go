//go:build linux

package nftabler

import (
	"context"
	"os/exec"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/nftables"
)

// Cleanup deletes all rules created by Nftabler; it's intended to be used
// during startup, to clean up rules created by an old incarnation of the daemon
// after switching to a different Firewaller implementation.
func Cleanup(ctx context.Context, config firewaller.Config) {
	if config.IPv4 {
		if err := exec.Command("nft", "delete", "table", string(nftables.IPv4), dockerTable).Run(); err != nil {
			log.G(ctx).WithError(err).Info("Deleting nftables IPv4 rules")
		} else {
			log.G(ctx).Info("Deleted nftables IPv4 rules")
		}
	}
	if config.IPv6 {
		if err := exec.Command("nft", "delete", "table", string(nftables.IPv6), dockerTable).Run(); err != nil {
			log.G(ctx).WithError(err).Info("Deleting nftables IPv6 rules")
		} else {
			log.G(ctx).Info("Deleted nftables IPv6 rules")
		}
	}
}

func (nft *Nftabler) SetFirewallCleaner(fc firewaller.FirewallCleaner) {
	nft.cleaner = fc
}

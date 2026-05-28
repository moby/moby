//go:build linux

package nftabler

import (
	"bytes"
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
		tryCleanup(ctx, nftables.IPv4, "IPv4")
	}
	if config.IPv6 {
		tryCleanup(ctx, nftables.IPv6, "IPv6")
	}
}

func tryCleanup(ctx context.Context, family nftables.Family, label string) {
	cmd := exec.CommandContext(ctx, "nft", "delete", "table", string(family), dockerTable)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// May not exist ("Error: Could not process rule: No such file or directory")
		log.G(ctx).WithFields(log.Fields{
			"error":  err,
			"output": string(bytes.TrimRight(out, "\n ^")), // remove "^^^^^" added in nft's error message.
		}).Info("Deleting nftables " + label + " rules")
		return
	}

	log.G(ctx).Info("Deleted nftables " + label + " rules")
}

func (nft *Nftabler) SetFirewallCleaner(fc firewaller.FirewallCleaner) {
	nft.cleaner = fc
}

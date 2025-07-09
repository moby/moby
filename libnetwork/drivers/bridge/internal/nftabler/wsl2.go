//go:build linux

package nftabler

import (
	"context"
	"fmt"

	"github.com/docker/docker/libnetwork/internal/nftables"
)

// mirroredWSL2Workaround adds IPv4 NAT rule if docker's host Linux appears to
// be a guest running under WSL2 in with mirrored mode networking.
// https://learn.microsoft.com/en-us/windows/wsl/networking#mirrored-mode-networking
//
// Without mirrored mode networking, or for a packet sent from Linux, packets
// sent to 127.0.0.1 are processed as outgoing - they hit the nat-OUTPUT chain,
// which does not jump to the nat-DOCKER chain because the rule has an exception
// for "-d 127.0.0.0/8". The default action on the nat-OUTPUT chain is ACCEPT (by
// default), so the packet is delivered to 127.0.0.1 on lo, where docker-proxy
// picks it up and acts as a man-in-the-middle; it receives the packet and
// re-sends it to the container (or acks a SYN and sets up a second TCP
// connection to the container). So, the container sees packets arrive with a
// source address belonging to the network's bridge, and it is able to reply to
// that address.
//
// In WSL2's mirrored networking mode, Linux has a loopback0 device as well as lo
// (which owns 127.0.0.1 as normal). Packets sent to 127.0.0.1 from Windows to a
// server listening on Linux's 127.0.0.1 are delivered via loopback0, and
// processed as packets arriving from outside the Linux host (which they are).
//
// So, these packets hit the nat-PREROUTING chain instead of nat-OUTPUT. It would
// normally be impossible for a packet ->127.0.0.1 to arrive from outside the
// host, so the nat-PREROUTING jump to nat-DOCKER has no exception for it. The
// packet is processed by a per-bridge DNAT rule in that chain, so it is
// delivered directly to the container (not via docker-proxy) with source address
// 127.0.0.1, so the container can't respond.
//
// DNAT is normally skipped by RETURN rules in the nat-DOCKER chain for packets
// arriving from any other bridge network. Similarly, this function adds (or
// removes) a rule to RETURN early for packets delivered via loopback0 with
// destination 127.0.0.0/8.
func mirroredWSL2Workaround(ctx context.Context, table nftables.TableRef) error {
	// WSL2 does not (currently) support Windows<->Linux communication via ::1.
	if table.Family() != nftables.IPv4 {
		return nil
	}
	chain := table.Chain(ctx, natChain)
	if !chain.IsValid() {
		return fmt.Errorf("failed to add loopback0 rule for WSL2, no '%s' chain", natChain)
	}
	return table.Chain(ctx, natChain).AppendRule(ctx,
		initialRuleGroup, `iifname "loopback0" ip daddr 127.0.0.0/8 counter return`)
}

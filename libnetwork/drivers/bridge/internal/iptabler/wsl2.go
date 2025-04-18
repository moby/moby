//go:build linux

package iptabler

import (
	"context"
	"errors"
	"os"

	"github.com/containerd/log"
	"github.com/docker/docker/internal/nlwrap"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/vishvananda/netlink"
)

// Path to the executable installed in Linux under WSL2 that reports on
// WSL config. https://github.com/microsoft/WSL/releases/tag/2.0.4
// Can be modified by tests.
var wslinfoPath = "/usr/bin/wslinfo"

// mirroredWSL2Workaround adds or removes an IPv4 NAT rule, depending on whether
// docker's host Linux appears to be a guest running under WSL2 in with mirrored
// mode networking.
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
func mirroredWSL2Workaround(ipv iptables.IPVersion, hairpin bool) error {
	// WSL2 does not (currently) support Windows<->Linux communication via ::1.
	if ipv != iptables.IPv4 {
		return nil
	}
	return programChainRule(mirroredWSL2Rule(), "WSL2 loopback", shouldInsertMirroredWSL2Rule(hairpin))
}

// shouldInsertMirroredWSL2Rule returns true if the NAT rule for mirrored WSL2 workaround
// is required. It is required if:
//   - the userland proxy is running. If not, there's nothing on the host to catch
//     the packet, so the loopback0 rule as wouldn't be useful. However, without
//     the workaround, with improvements in WSL2 v2.3.11, and without userland proxy
//     running - no workaround is needed, the normal DNAT/masquerading works.
//   - and, the host Linux appears to be running under Windows WSL2 with mirrored
//     mode networking.
func shouldInsertMirroredWSL2Rule(hairpin bool) bool {
	if hairpin {
		return false
	}
	return isRunningUnderWSL2MirroredMode()
}

// isRunningUnderWSL2MirroredMode returns true if the host Linux appears to be
// running under Windows WSL2 with mirrored mode networking. If a loopback0
// device exists, and there's an executable at /usr/bin/wslinfo, infer that
// this is WSL2 with mirrored networking. ("wslinfo --networking-mode" reports
// "mirrored", but applying the workaround for WSL2's loopback device when it's
// not needed is low risk, compared with executing wslinfo with dockerd's
// elevated permissions.)
func isRunningUnderWSL2MirroredMode() bool {
	if _, err := nlwrap.LinkByName("loopback0"); err != nil {
		if !errors.As(err, &netlink.LinkNotFoundError{}) {
			log.G(context.TODO()).WithError(err).Warn("Failed to check for WSL interface")
		}
		return false
	}
	stat, err := os.Stat(wslinfoPath)
	if err != nil {
		return false
	}
	return stat.Mode().IsRegular() && (stat.Mode().Perm()&0o111) != 0
}

func mirroredWSL2Rule() iptables.Rule {
	return iptables.Rule{
		IPVer: iptables.IPv4,
		Table: iptables.Nat,
		Chain: dockerChain,
		Args:  []string{"-i", "loopback0", "-d", "127.0.0.0/8", "-j", "RETURN"},
	}
}

//go:build linux

package overlay

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/overlay/overlayutils"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/nftables"
)

const (
	nftOverlayTable    = "docker-overlay"
	nftEncOutChainName = "enc-out"
	nftEncInChainName  = "enc-in"
	nftEncVNSetName    = "encrypted-vnis"
	nftEncVNIExpr      = "@th,96,24"
)

// ensureOverlayEncNftTable returns the overlay encryption nft table, running one-time setup on first use.
func (d *driver) ensureOverlayEncNftTable(ctx context.Context) (*nftables.Table, error) {
	d.overlayEncNftInitMu.Lock()
	defer d.overlayEncNftInitMu.Unlock()
	if d.overlayEncNftTable.IsValid() {
		return d.overlayEncNftTable, nil
	}

	v6, err := d.isIPv6Transport()
	if err != nil {
		return nil, err
	}
	fam := nftables.IPv4
	if v6 {
		fam = nftables.IPv6
	}

	if err := d.programMangleMarkSpoofProtection(false); err != nil {
		// Best-effort: it's safe for the stale iptables rule to remain
		// as it will not scrub the encryption mark from authentic VXLAN
		// packets from the kernel.
		log.G(ctx).WithError(err).Warn("Failed to clean up stale iptables local-spoof-protection rule")
	}

	t, err := nftables.NewTable(fam, nftOverlayTable)
	if err != nil {
		return nil, err
	}

	tm := nftables.Modifier{}
	tm.Create(nftables.Set{
		Name:        nftEncVNSetName,
		ElementType: nftables.Typeof(nftEncVNIExpr),
	})
	tm.Create(nftables.BaseChain{
		Name:      nftEncOutChainName,
		ChainType: nftables.BaseChainTypeRoute,
		Hook:      nftables.BaseChainHookOutput,
		Priority:  nftables.BaseChainPriorityMangle,
		Policy:    nftables.BaseChainPolicyAccept,
	})
	tm.Create(nftables.BaseChain{
		Name:      nftEncInChainName,
		ChainType: nftables.BaseChainTypeFilter,
		Hook:      nftables.BaseChainHookInput,
		Priority:  nftables.BaseChainPriorityRaw,
		Policy:    nftables.BaseChainPolicyAccept,
	})

	port := strconv.FormatUint(uint64(overlayutils.VXLANUDPPort()), 10)
	m := fmt.Sprintf("0x%x", mark)
	// Our XFRM policy for encryption matches on packets addressed to the
	// VXLAN UDP port which are marked with our encryption mark. Protect
	// against encrypting spoofed VXLAN packets by ensuring that packets
	// addressed to the VXLAN UDP port are not marked for encryption unless
	// they were sent by the kernel. We only need to explicitly guard
	// against spoofed packets from the host netns as containers cannot
	// forge packets with the mark already set (packet marks are scrubbed
	// when the packet crosses a netns boundary) and we cannot accidentally
	// mark them with our ruleset as forwarded packets are not evaluated by
	// OUTPUT hooks at all.
	tm.Create(nftables.Rule{
		Chain: nftEncOutChainName,
		Rule: []string{
			"udp dport", port,
			// Match packets associated with a socket in the same
			// netns, which is only the case for packets sent from
			// userspace in the same netns. `meta skuid` expressions
			// will only match a packet associated with a socket
			// which belongs to the netns the hook is running in.
			// VXLAN packets are associated with the socket of the
			// packets they encapsulate. Authentic VXLAN packets
			// will not match as they are associated with the
			// foreign container-namespace sockets of the packets
			// they encapsulate or with no socket at all (always the
			// case on Linux v4.18 and earlier, which scrubbed
			// socket associations when crossing netns boundaries).
			//
			// Note that this idiom, unlike the iptables
			//
			//     -m owner --socket-exists
			//
			// match, does not have an inverse -- we cannot write a
			// single rule which applies statements when the packet
			// is not associated with a socket in the current netns.
			// Rule evaluation will short-circuit when a
			// `meta skuid` expression is evaluated on a packet not
			// associated with a local socket, like how the
			// `udp dport` expression short-circuits the rule when
			// the packet is not UDP.
			//
			// Match the full UID range (excluding (uid_t)-1, the
			// invalid UID sentinel) as we only care about the side
			// effect, irrespective of the UID when a socket exists.
			"meta skuid 0-0xfffffffe",
			"counter",
			"goto {",
			// Block processes in the host netns with CAP_NET_RAW
			// from using setsockopt(SO_MARK) to encrypt forged
			// packets.
			"meta mark", m,
			"counter",
			"meta mark set 0",
			";",
			"}",
		},
	})

	tm.Create(nftables.Rule{
		Chain: nftEncOutChainName,
		Rule: []string{
			"udp dport", port,
			nftEncVNIExpr,
			"@" + nftEncVNSetName,
			"counter",
			"meta mark set", m,
		},
	})
	tm.Create(nftables.Rule{
		Chain: nftEncInChainName,
		Rule: []string{
			"meta secpath missing",
			"udp dport", port,
			nftEncVNIExpr,
			"@" + nftEncVNSetName,
			"counter",
			"drop",
		},
	})

	if err := t.Apply(ctx, tm); err != nil {
		_ = t.Close()
		return nil, err
	}

	d.overlayEncNftTable = t
	return t, nil
}

func (d *driver) programOverlayEncVNINft(ctx context.Context, vni uint32, encrypted bool) error {
	// Attempt to clean up stale iptables rules from an old incarnation of
	// the daemon which could clash with the nftables ruleset.
	cleanupErr := errors.Join(d.programInput(vni, false), d.programMangle(vni, false))
	if cleanupErr != nil {
		log.G(ctx).WithError(cleanupErr).Infof("Failed to clean up stale iptables rules for VNI %d", vni)
	}

	t, err := d.ensureOverlayEncNftTable(ctx)
	if err != nil {
		return err
	}

	tm := nftables.Modifier{}
	se := nftables.SetElement{
		SetName:    nftEncVNSetName,
		Element:    fmt.Sprintf("0x%06x", vni&0xffffff),
		Idempotent: true,
	}
	if encrypted {
		tm.Create(se)
	} else {
		tm.Delete(se)
	}
	return t.Apply(ctx, tm)
}

// cleanupNft deletes all nftables rules created by the driver. It's intended to
// be used during startup, to clean up rules created by an old incarnation of
// the daemon after switching to a different firewall backend.
func (d *driver) cleanupNft(ctx context.Context) {
	v6, err := d.isIPv6Transport()
	if err != nil {
		log.G(ctx).WithError(err).Error("Deleting overlay encryption nftables rules")
		return
	}
	fam := nftables.IPv4
	if v6 {
		fam = nftables.IPv6
	}
	if err := nftables.RunCmd(ctx, fmt.Appendf(nil, "delete table %s %s", fam, nftOverlayTable)); err != nil {
		log.G(ctx).WithError(err).Info("Deleting overlay encryption nftables rules")
		return
	}
	log.G(ctx).Info("Deleted overlay encryption nftables rules")
}

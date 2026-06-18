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
func (d *driver) ensureOverlayEncNftTable(ctx context.Context) (nftables.Table, error) {
	d.overlayEncNftInitMu.Lock()
	defer d.overlayEncNftInitMu.Unlock()
	if d.overlayEncNftTable.IsValid() {
		return d.overlayEncNftTable, nil
	}

	v6, err := d.isIPv6Transport()
	if err != nil {
		return nftables.Table{}, err
	}
	fam := nftables.IPv4
	if v6 {
		fam = nftables.IPv6
	}
	t, err := nftables.NewTable(fam, nftOverlayTable)
	if err != nil {
		return nftables.Table{}, err
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
	tm.Create(nftables.Rule{
		Chain: nftEncOutChainName,
		Rule: []string{
			"udp dport", port,
			nftEncVNIExpr,
			"@" + nftEncVNSetName,
			"counter",
			"meta mark set", fmt.Sprintf("0x%x", mark),
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
		return nftables.Table{}, err
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

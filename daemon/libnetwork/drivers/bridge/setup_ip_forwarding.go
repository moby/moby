//go:build linux

package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/containerd/log"
	"github.com/docker/docker/daemon/libnetwork/datastore"
	"github.com/docker/docker/daemon/libnetwork/drivers/bridge/internal/firewaller"
)

const (
	ipv4ForwardConf        = "/proc/sys/net/ipv4/ip_forward"
	ipv6ForwardConfDefault = "/proc/sys/net/ipv6/conf/default/forwarding"
	ipv6ForwardConfAll     = "/proc/sys/net/ipv6/conf/all/forwarding"
)

func (d *driver) setupIPv4Forwarding(ctx context.Context) (retErr error) {
	changed, err := configureIPForwarding(ipv4ForwardConf, '1')
	if err != nil {
		return err
	}
	if changed {
		defer func() {
			if retErr != nil {
				if _, err := configureIPForwarding(ipv4ForwardConf, '0'); err != nil {
					log.G(ctx).WithError(err).Error("Cannot disable IPv4 forwarding")
				}
			}
		}()

		if err := d.setFilterForwardDrop(ctx, firewaller.IPv4); err != nil {
			return err
		}
	}
	return nil
}

func (d *driver) setupIPv6Forwarding(ctx context.Context) (retErr error) {
	// Set IPv6 default.forwarding, if needed.
	changedDef, err := configureIPForwarding(ipv6ForwardConfDefault, '1')
	if err != nil {
		return err
	}
	if changedDef {
		defer func() {
			if retErr != nil {
				if _, err := configureIPForwarding(ipv6ForwardConfDefault, '0'); err != nil {
					log.G(ctx).WithError(err).Error("Cannot disable IPv6 default.forwarding")
				}
			}
		}()
	}

	// Set IPv6 all.forwarding, if needed.
	changedAll, err := configureIPForwarding(ipv6ForwardConfAll, '1')
	if err != nil {
		return err
	}
	if changedAll {
		defer func() {
			if retErr != nil {
				if _, err := configureIPForwarding(ipv6ForwardConfAll, '0'); err != nil {
					log.G(ctx).WithError(err).Error("Cannot disable IPv6 all.forwarding")
				}
			}
		}()
	}

	if changedAll || changedDef {
		if err := d.setFilterForwardDrop(ctx, firewaller.IPv6); err != nil {
			return err
		}
	}

	return nil
}

func configureIPForwarding(file string, val byte) (changed bool, _ error) {
	data, err := os.ReadFile(file)
	if err != nil || len(data) == 0 {
		return false, fmt.Errorf("cannot read IP forwarding setup from '%s': %w", file, err)
	}
	if len(data) == 0 {
		return false, fmt.Errorf("cannot read IP forwarding setup from '%s': 0 bytes", file)
	}
	if data[0] == val {
		return false, nil
	}
	if err := os.WriteFile(file, []byte{val, '\n'}, 0o644); err != nil {
		return false, fmt.Errorf("failed to set IP forwarding '%s' = '%c': %w", file, val, err)
	}
	return true, nil
}

const (
	ipForwardingEnabledKeyPrefix = "ip_forwarding_enabled"
	ipForwarding4                = "ipv4"
	ipForwarding6                = "ipv6"
)

// filterForwardDrop being present in the store for a given IP version
// indicates that the filter-FORWARD policy has been set to drop by the
// daemon. So, future incarnations of the daemon should also set it.
type filterForwardDrop struct {
	ipv string

	// For the datastore...
	dbIndex  uint64
	dbExists bool
}

// setFilterForwardDrop tells the firewaller to set the filter-FORWARD policy to
// "drop", and remembers that in the store so that future incarnations of the
// daemon will also set it.
func (d *driver) setFilterForwardDrop(ctx context.Context, ipv firewaller.IPVersion) error {
	ipvKey := ipForwarding4
	if ipv == firewaller.IPv6 {
		ipvKey = ipForwarding6
	}
	if err := d.storeUpdate(ctx, &filterForwardDrop{ipv: ipvKey}); err != nil && !errors.Is(err, datastore.ErrKeyModified) {
		log.G(ctx).WithError(err).Error("Cannot persist IP forwarding enabled flag")
	}

	if d.config.DisableFilterForwardDrop {
		return nil
	}
	if ipv == firewaller.IPv4 && !d.config.EnableIPTables {
		return nil
	}
	if ipv == firewaller.IPv6 && !d.config.EnableIP6Tables {
		return nil
	}
	if err := d.firewaller.FilterForwardDrop(ctx, ipv); err != nil {
		return err
	}
	log.G(ctx).WithField("ipv", ipv).Warn("IP forwarding policy set to 'drop', use '--ip-forward-no-drop' to disable")
	return nil
}

// initForwardingPolicy checks whether an earlier incarnation of the daemon has set
// the filter-FORWARD policy to "drop". If it has, it also sets the policy to drop.
//
// (The policy is set when IP forwarding is enabled. But is only likely to happen when
// the daemon is first started. On restarts with nftables, the policy is lost when
// the nftables table is reconstructed. So, it needs to be re-set here.)
func (d *driver) initForwardingPolicy(ctx context.Context) error {
	ffds, err := d.store.List(&filterForwardDrop{})
	if err != nil {
		if errors.Is(err, datastore.ErrKeyNotFound) {
			return nil
		}
		return err
	}
	for _, i := range ffds {
		ffd := i.(*filterForwardDrop)
		ipv := firewaller.IPv4
		if ffd.ipv == ipForwarding6 {
			ipv = firewaller.IPv6
		}
		if err := d.setFilterForwardDrop(ctx, ipv); err != nil {
			return fmt.Errorf("setting %s forwarding policy to drop: %w", ffd.ipv, err)
		}
	}
	return nil
}

func (ife *filterForwardDrop) Key() []string {
	return []string{ipForwardingEnabledKeyPrefix, ife.ipv}
}

func (ife *filterForwardDrop) KeyPrefix() []string {
	return []string{ipForwardingEnabledKeyPrefix}
}

func (ife *filterForwardDrop) Value() []byte {
	b, err := json.Marshal(ife)
	if err != nil {
		return nil
	}
	return b
}

func (ife *filterForwardDrop) SetValue(value []byte) error {
	return json.Unmarshal(value, ife)
}

func (ife *filterForwardDrop) Index() uint64 {
	return ife.dbIndex
}

func (ife *filterForwardDrop) SetIndex(index uint64) {
	ife.dbIndex = index
	ife.dbExists = true
}

func (ife *filterForwardDrop) Exists() bool {
	return ife.dbExists
}

func (ife *filterForwardDrop) Skip() bool {
	return false
}

func (ife *filterForwardDrop) New() datastore.KVObject {
	return &filterForwardDrop{}
}

func (ife *filterForwardDrop) CopyTo(o datastore.KVObject) error {
	dstNcfg := o.(*filterForwardDrop)
	*dstNcfg = *ife
	return nil
}

func (ife *filterForwardDrop) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"ID": ife.ipv,
	})
}

func (ife *filterForwardDrop) UnmarshalJSON(b []byte) error {
	m := map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	ife.ipv = m["ID"].(string)
	return nil
}

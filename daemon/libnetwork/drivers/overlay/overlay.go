//go:build linux

package overlay

//go:generate protoc -I=. -I=../../../../vendor/ --gogofaster_out=import_path=github.com/docker/docker/daemon/libnetwork/drivers/overlay:. overlay.proto

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/discoverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/scope"
)

const (
	NetworkType  = "overlay"
	vethPrefix   = "veth"
	vethLen      = len(vethPrefix) + 7
	vxlanEncap   = 50
	secureOption = "encrypted"
)

var (
	_ discoverapi.Discover   = (*driver)(nil)
	_ driverapi.TableWatcher = (*driver)(nil)
)

type driver struct {
	// Immutable; mu does not need to be held when accessing these fields.

	config map[string]any
	initOS sync.Once

	// encrMu guards secMap and keys,
	// and synchronizes the application of encryption parameters
	// to the kernel.
	//
	// This mutex is above mu in the lock hierarchy.
	// Do not lock any locks aside from mu while holding encrMu.
	encrMu sync.Mutex
	secMap encrMap
	keys   []*key

	// mu must be held when accessing the fields which follow it
	// in the struct definition.
	//
	// This mutex is at the bottom of the lock hierarchy:
	// do not lock any other locks while holding it.
	mu               sync.Mutex
	bindAddress      netip.Addr
	advertiseAddress netip.Addr
	networks         networkTable
}

// Register registers a new instance of the overlay driver.
func Register(r driverapi.Registerer, config map[string]any) error {
	d := &driver{
		networks: networkTable{},
		secMap:   encrMap{},
		config:   config,
	}
	return r.RegisterDriver(NetworkType, d, driverapi.Capability{
		DataScope:         scope.Global,
		ConnectivityScope: scope.Global,
	})
}

func (d *driver) configure() error {
	// Apply OS specific kernel configs if needed
	d.initOS.Do(applyOStweaks)

	return nil
}

func (d *driver) Type() string {
	return NetworkType
}

func (d *driver) IsBuiltIn() bool {
	return true
}

// isIPv6Transport reports whether the outer Layer-3 transport for VXLAN datagrams is IPv6.
func (d *driver) isIPv6Transport() (bool, error) {
	// Infer whether remote peers' virtual tunnel endpoints will be IPv4 or IPv6
	// from the address family of our own advertise address. This is a
	// reasonable inference to make as Linux VXLAN links do not support
	// mixed-address-family remote peers.
	if !d.advertiseAddress.IsValid() {
		return false, errors.New("overlay: cannot determine address family of transport: the local data-plane address is not currently known")
	}
	return d.advertiseAddress.Is6(), nil
}

func (d *driver) nodeJoin(data discoverapi.NodeDiscoveryData) error {
	if data.Self {
		advAddr, _ := netip.ParseAddr(data.Address)
		bindAddr, _ := netip.ParseAddr(data.BindAddress)
		if !advAddr.IsValid() {
			return errors.New("invalid discovery data")
		}
		d.mu.Lock()
		d.advertiseAddress = advAddr
		d.bindAddress = bindAddr
		d.mu.Unlock()
	}
	return nil
}

// DiscoverNew is a notification for a new discovery event, such as a new node joining a cluster
func (d *driver) DiscoverNew(dType discoverapi.DiscoveryType, data any) error {
	switch dType {
	case discoverapi.NodeDiscovery:
		nodeData, ok := data.(discoverapi.NodeDiscoveryData)
		if !ok {
			return fmt.Errorf("invalid discovery data type: %T", data)
		}
		return d.nodeJoin(nodeData)
	case discoverapi.EncryptionKeysConfig:
		encrData, ok := data.(discoverapi.DriverEncryptionConfig)
		if !ok {
			return errors.New("invalid encryption key notification data")
		}
		keys := make([]*key, 0, len(encrData.Keys))
		for i := 0; i < len(encrData.Keys); i++ {
			k := &key{
				value: encrData.Keys[i],
				tag:   uint32(encrData.Tags[i]),
			}
			keys = append(keys, k)
		}
		if err := d.setKeys(keys); err != nil {
			log.G(context.TODO()).Warn(err)
		}
	case discoverapi.EncryptionKeysUpdate:
		var newKey, delKey, priKey *key
		encrData, ok := data.(discoverapi.DriverEncryptionUpdate)
		if !ok {
			return errors.New("invalid encryption key notification data")
		}
		if encrData.Key != nil {
			newKey = &key{
				value: encrData.Key,
				tag:   uint32(encrData.Tag),
			}
		}
		if encrData.Primary != nil {
			priKey = &key{
				value: encrData.Primary,
				tag:   uint32(encrData.PrimaryTag),
			}
		}
		if encrData.Prune != nil {
			delKey = &key{
				value: encrData.Prune,
				tag:   uint32(encrData.PruneTag),
			}
		}
		if err := d.updateKeys(newKey, priKey, delKey); err != nil {
			return err
		}
	default:
	}
	return nil
}

// DiscoverDelete is a notification for a discovery delete event, such as a node leaving a cluster
func (d *driver) DiscoverDelete(dType discoverapi.DiscoveryType, data any) error {
	return nil
}

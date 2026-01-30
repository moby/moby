//go:build linux

package overlay

//go:generate protoc -I=. -I=../../../../vendor/ --gogofaster_out=import_path=github.com/docker/docker/daemon/libnetwork/drivers/overlay:. overlay.proto

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"

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
func Register(ctx context.Context, r driverapi.Registerer) error {
	d := &driver{
		networks: networkTable{},
		secMap:   encrMap{},
	}
	return r.RegisterDriver(ctx, NetworkType, d, driverapi.Capability{
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
func (d *driver) DiscoverNew(ctx context.Context, dType discoverapi.DiscoveryType, data any) error {
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
			return fmt.Errorf("invalid encryption key notification data type: %T", data)
		}
		return d.setKeys(ctx, encrData)
	case discoverapi.EncryptionKeysUpdate:
		encrData, ok := data.(discoverapi.DriverEncryptionUpdate)
		if !ok {
			return fmt.Errorf("invalid encryption key notification data type: %T", data)
		}
		return d.updateKeys(ctx, encrData)
	default:
		return nil
	}
}

// DiscoverDelete is a notification for a discovery delete event, such as a node leaving a cluster
func (d *driver) DiscoverDelete(ctx context.Context, dType discoverapi.DiscoveryType, data any) error {
	return nil
}

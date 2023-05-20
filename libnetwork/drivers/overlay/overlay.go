//go:build linux

package overlay

//go:generate protoc -I.:../../Godeps/_workspace/src/github.com/gogo/protobuf  --gogo_out=import_path=github.com/docker/docker/libnetwork/drivers/overlay,Mgogoproto/gogo.proto=github.com/gogo/protobuf/gogoproto:. overlay.proto

import (
	"fmt"
	"sync"

	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/discoverapi"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/sirupsen/logrus"
)

const (
	networkType  = "overlay"
	vethPrefix   = "veth"
	vethLen      = len(vethPrefix) + 7
	vxlanEncap   = 50
	secureOption = "encrypted"
)

type driver struct {
	bindAddress      string
	advertiseAddress string
	config           map[string]interface{}
	peerDb           peerNetworkMap
	secMap           *encrMap
	networks         networkTable
	initOS           sync.Once
	localJoinOnce    sync.Once
	keys             []*key
	peerOpMu         sync.Mutex
	sync.Mutex
}

// Register registers a new instance of the overlay driver.
func Register(r driverapi.Registerer, config map[string]interface{}) error {
	c := driverapi.Capability{
		DataScope:         datastore.GlobalScope,
		ConnectivityScope: datastore.GlobalScope,
	}
	d := &driver{
		networks: networkTable{},
		peerDb: peerNetworkMap{
			mp: map[string]*peerMap{},
		},
		secMap: &encrMap{nodes: map[string][]*spi{}},
		config: config,
	}

	return r.RegisterDriver(networkType, d, c)
}

func (d *driver) configure() error {
	// Apply OS specific kernel configs if needed
	d.initOS.Do(applyOStweaks)

	return nil
}

func (d *driver) Type() string {
	return networkType
}

func (d *driver) IsBuiltIn() bool {
	return true
}

func (d *driver) nodeJoin(advertiseAddress, bindAddress string, self bool) {
	if self {
		d.Lock()
		d.advertiseAddress = advertiseAddress
		d.bindAddress = bindAddress
		d.Unlock()

		// If containers are already running on this network update the
		// advertise address in the peerDB
		d.localJoinOnce.Do(func() {
			d.peerDBUpdateSelf()
		})
	}
}

// DiscoverNew is a notification for a new discovery event, such as a new node joining a cluster
func (d *driver) DiscoverNew(dType discoverapi.DiscoveryType, data interface{}) error {
	switch dType {
	case discoverapi.NodeDiscovery:
		nodeData, ok := data.(discoverapi.NodeDiscoveryData)
		if !ok || nodeData.Address == "" {
			return fmt.Errorf("invalid discovery data")
		}
		d.nodeJoin(nodeData.Address, nodeData.BindAddress, nodeData.Self)
	case discoverapi.EncryptionKeysConfig:
		encrData, ok := data.(discoverapi.DriverEncryptionConfig)
		if !ok {
			return fmt.Errorf("invalid encryption key notification data")
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
			logrus.Warn(err)
		}
	case discoverapi.EncryptionKeysUpdate:
		var newKey, delKey, priKey *key
		encrData, ok := data.(discoverapi.DriverEncryptionUpdate)
		if !ok {
			return fmt.Errorf("invalid encryption key notification data")
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
func (d *driver) DiscoverDelete(dType discoverapi.DiscoveryType, data interface{}) error {
	return nil
}

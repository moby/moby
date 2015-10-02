package overlay

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libkv/store"
	"github.com/docker/libnetwork/config"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/idm"
	"github.com/docker/libnetwork/netlabel"
	"github.com/hashicorp/serf/serf"
)

const (
	networkType  = "overlay"
	vethPrefix   = "veth"
	vethLen      = 7
	vxlanIDStart = 256
	vxlanIDEnd   = 1000
	vxlanPort    = 4789
	vxlanVethMTU = 1450
)

type driver struct {
	eventCh      chan serf.Event
	notifyCh     chan ovNotify
	exitCh       chan chan struct{}
	bindAddress  string
	neighIP      string
	config       map[string]interface{}
	peerDb       peerNetworkMap
	serfInstance *serf.Serf
	networks     networkTable
	store        datastore.DataStore
	ipAllocator  *idm.Idm
	vxlanIdm     *idm.Idm
	once         sync.Once
	joinOnce     sync.Once
	sync.Mutex
}

var (
	bridgeSubnet, bridgeIP *net.IPNet
	once                   sync.Once
	bridgeSubnetInt        uint32
)

func onceInit() {
	var err error
	_, bridgeSubnet, err = net.ParseCIDR("172.21.0.0/16")
	if err != nil {
		panic("could not parse cid 172.21.0.0/16")
	}

	bridgeSubnetInt = binary.BigEndian.Uint32(bridgeSubnet.IP.To4())

	ip, subnet, err := net.ParseCIDR("172.21.255.254/16")
	if err != nil {
		panic("could not parse cid 172.21.255.254/16")
	}

	bridgeIP = &net.IPNet{
		IP:   ip,
		Mask: subnet.Mask,
	}
}

// Init registers a new instance of overlay driver
func Init(dc driverapi.DriverCallback, config map[string]interface{}) error {
	once.Do(onceInit)

	c := driverapi.Capability{
		DataScope: datastore.GlobalScope,
	}

	d := &driver{
		networks: networkTable{},
		peerDb: peerNetworkMap{
			mp: map[string]peerMap{},
		},
		config: config,
	}

	return dc.RegisterDriver(networkType, d, c)
}

// Fini cleans up the driver resources
func Fini(drv driverapi.Driver) {
	d := drv.(*driver)

	if d.exitCh != nil {
		waitCh := make(chan struct{})

		d.exitCh <- waitCh

		<-waitCh
	}
}

func (d *driver) configure() error {
	var err error

	if len(d.config) == 0 {
		return nil
	}

	d.once.Do(func() {
		provider, provOk := d.config[netlabel.KVProvider]
		provURL, urlOk := d.config[netlabel.KVProviderURL]

		if provOk && urlOk {
			cfg := &config.DatastoreCfg{
				Client: config.DatastoreClientCfg{
					Provider: provider.(string),
					Address:  provURL.(string),
				},
			}
			provConfig, confOk := d.config[netlabel.KVProviderConfig]
			if confOk {
				cfg.Client.Config = provConfig.(*store.Config)
			}
			d.store, err = datastore.NewDataStore(cfg)
			if err != nil {
				err = fmt.Errorf("failed to initialize data store: %v", err)
				return
			}
		}

		d.vxlanIdm, err = idm.New(d.store, "vxlan-id", vxlanIDStart, vxlanIDEnd)
		if err != nil {
			err = fmt.Errorf("failed to initialize vxlan id manager: %v", err)
			return
		}

		d.ipAllocator, err = idm.New(d.store, "ipam-id", 1, 0xFFFF-2)
		if err != nil {
			err = fmt.Errorf("failed to initalize ipam id manager: %v", err)
			return
		}
	})

	return err
}

func (d *driver) Type() string {
	return networkType
}

func (d *driver) nodeJoin(node string, self bool) {
	if self && !d.isSerfAlive() {
		d.Lock()
		d.bindAddress = node
		d.Unlock()
		err := d.serfInit()
		if err != nil {
			logrus.Errorf("initializing serf instance failed: %v", err)
			return
		}
	}

	d.Lock()
	if !self {
		d.neighIP = node
	}
	neighIP := d.neighIP
	d.Unlock()

	if d.serfInstance != nil && neighIP != "" {
		var err error
		d.joinOnce.Do(func() {
			err = d.serfJoin(neighIP)
			if err == nil {
				d.pushLocalDb()
			}
		})
		if err != nil {
			logrus.Errorf("joining serf neighbor %s failed: %v", node, err)
			d.Lock()
			d.joinOnce = sync.Once{}
			d.Unlock()
			return
		}
	}
}

func (d *driver) pushLocalEndpointEvent(action, nid, eid string) {
	if !d.isSerfAlive() {
		return
	}
	d.notifyCh <- ovNotify{
		action: "join",
		nid:    nid,
		eid:    eid,
	}
}

// DiscoverNew is a notification for a new discovery event, such as a new node joining a cluster
func (d *driver) DiscoverNew(dType driverapi.DiscoveryType, data interface{}) error {
	if dType == driverapi.NodeDiscovery {
		nodeData, ok := data.(driverapi.NodeDiscoveryData)
		if !ok || nodeData.Address == "" {
			return fmt.Errorf("invalid discovery data")
		}
		d.nodeJoin(nodeData.Address, nodeData.Self)
	}
	return nil
}

// DiscoverDelete is a notification for a discovery delete event, such as a node leaving a cluster
func (d *driver) DiscoverDelete(dType driverapi.DiscoveryType, data interface{}) error {
	return nil
}

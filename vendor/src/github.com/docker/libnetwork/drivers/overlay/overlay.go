package overlay

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"

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
	ifaceName    string
	neighIP      string
	config       map[string]interface{}
	peerDb       peerNetworkMap
	serfInstance *serf.Serf
	networks     networkTable
	store        datastore.DataStore
	ipAllocator  *idm.Idm
	vxlanIdm     *idm.Idm
	sync.Once
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
	var onceDone bool
	var err error

	if len(d.config) == 0 {
		return nil
	}

	d.Do(func() {
		onceDone = true

		if ifaceName, ok := d.config[netlabel.OverlayBindInterface]; ok {
			d.ifaceName = ifaceName.(string)
		}

		if neighIP, ok := d.config[netlabel.OverlayNeighborIP]; ok {
			d.neighIP = neighIP.(string)
		}

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

		err = d.serfInit()
		if err != nil {
			err = fmt.Errorf("initializing serf instance failed: %v", err)
		}

	})

	return err
}

func (d *driver) Type() string {
	return networkType
}

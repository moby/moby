package overlay

//go:generate protoc -I.:../../Godeps/_workspace/src/github.com/gogo/protobuf  --gogo_out=import_path=github.com/docker/libnetwork/drivers/overlay,Mgogoproto/gogo.proto=github.com/gogo/protobuf/gogoproto:. overlay.proto

import (
	"fmt"
	"net"
	"sync"

	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/discoverapi"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/idm"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/types"
	"github.com/hashicorp/serf/serf"
)

const (
	networkType  = "overlay"
	vethPrefix   = "veth"
	vethLen      = 7
	vxlanIDStart = 4096
	vxlanIDEnd   = (1 << 24) - 1
	vxlanPort    = 4789
	vxlanEncap   = 50
	secureOption = "encrypted"
)

var initVxlanIdm = make(chan (bool), 1)

type driver struct {
	eventCh          chan serf.Event
	notifyCh         chan ovNotify
	exitCh           chan chan struct{}
	bindAddress      string
	advertiseAddress string
	neighIP          string
	config           map[string]interface{}
	serfInstance     *serf.Serf
	networks         networkTable
	store            datastore.DataStore
	localStore       datastore.DataStore
	vxlanIdm         *idm.Idm
	once             sync.Once
	joinOnce         sync.Once
	sync.Mutex
}

// Init registers a new instance of overlay driver
func Init(dc driverapi.DriverCallback, config map[string]interface{}) error {
	c := driverapi.Capability{
		DataScope: datastore.GlobalScope,
	}

	d := &driver{
		networks: networkTable{},
		config:   config,
	}

	if data, ok := config[netlabel.GlobalKVClient]; ok {
		var err error
		dsc, ok := data.(discoverapi.DatastoreConfigData)
		if !ok {
			return types.InternalErrorf("incorrect data in datastore configuration: %v", data)
		}
		d.store, err = datastore.NewDataStoreFromConfig(dsc)
		if err != nil {
			return types.InternalErrorf("failed to initialize data store: %v", err)
		}
	}

	if data, ok := config[netlabel.LocalKVClient]; ok {
		var err error
		dsc, ok := data.(discoverapi.DatastoreConfigData)
		if !ok {
			return types.InternalErrorf("incorrect data in datastore configuration: %v", data)
		}
		d.localStore, err = datastore.NewDataStoreFromConfig(dsc)
		if err != nil {
			return types.InternalErrorf("failed to initialize local data store: %v", err)
		}
	}

	d.restoreEndpoints()

	return dc.RegisterDriver(networkType, d, c)
}

// Endpoints are stored in the local store. Restore them and reconstruct the overlay sandbox
func (d *driver) restoreEndpoints() error {
	if d.localStore == nil {
		logrus.Warnf("Cannot restore overlay endpoints because local datastore is missing")
		return nil
	}
	kvol, err := d.localStore.List(datastore.Key(overlayEndpointPrefix), &endpoint{})
	if err != nil && err != datastore.ErrKeyNotFound {
		return fmt.Errorf("failed to read overlay endpoint from store: %v", err)
	}

	if err == datastore.ErrKeyNotFound {
		return nil
	}

	for _, kvo := range kvol {
		ep := kvo.(*endpoint)

		n := d.network(ep.nid)
		if n == nil || ep.remote {
			if !ep.remote {
				logrus.Debugf("Network (%s) not found for restored endpoint (%s)", ep.nid[0:7], ep.id[0:7])
				logrus.Debugf("Deleting stale overlay endpoint (%s) from store", ep.id[0:7])
			}

			hcsshim.HNSEndpointRequest("DELETE", ep.profileId, "")

			if err := d.deleteEndpointFromStore(ep); err != nil {
				logrus.Debugf("Failed to delete stale overlay endpoint (%s) from store", ep.id[0:7])
			}

			continue
		}

		n.addEndpoint(ep)
	}

	return nil
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
	if d.store == nil {
		return nil
	}

	if d.vxlanIdm == nil {
		return d.initializeVxlanIdm()
	}

	return nil
}

func (d *driver) initializeVxlanIdm() error {
	var err error

	initVxlanIdm <- true
	defer func() { <-initVxlanIdm }()

	if d.vxlanIdm != nil {
		return nil
	}

	d.vxlanIdm, err = idm.New(d.store, "vxlan-id", vxlanIDStart, vxlanIDEnd)
	if err != nil {
		return fmt.Errorf("failed to initialize vxlan id manager: %v", err)
	}

	return nil
}

func (d *driver) Type() string {
	return networkType
}

func (d *driver) IsBuiltIn() bool {
	return true
}

func validateSelf(node string) error {
	advIP := net.ParseIP(node)
	if advIP == nil {
		return fmt.Errorf("invalid self address (%s)", node)
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return fmt.Errorf("Unable to get interface addresses %v", err)
	}
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err == nil && ip.Equal(advIP) {
			return nil
		}
	}
	return fmt.Errorf("Multi-Host overlay networking requires cluster-advertise(%s) to be configured with a local ip-address that is reachable within the cluster", advIP.String())
}

func (d *driver) nodeJoin(advertiseAddress, bindAddress string, self bool) {
	if self && !d.isSerfAlive() {
		if err := validateSelf(advertiseAddress); err != nil {
			logrus.Errorf("%s", err.Error())
		}

		d.Lock()
		d.advertiseAddress = advertiseAddress
		d.bindAddress = bindAddress
		d.Unlock()

		// If there is no cluster store there is no need to start serf.
		if d.store != nil {
			err := d.serfInit()
			if err != nil {
				logrus.Errorf("initializing serf instance failed: %v", err)
				return
			}
		}
	}

	d.Lock()
	if !self {
		d.neighIP = advertiseAddress
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
			logrus.Errorf("joining serf neighbor %s failed: %v", advertiseAddress, err)
			d.Lock()
			d.joinOnce = sync.Once{}
			d.Unlock()
			return
		}
	}
}

func (d *driver) pushLocalEndpointEvent(action, nid, eid string) {
	n := d.network(nid)
	if n == nil {
		logrus.Debugf("Error pushing local endpoint event for network %s", nid)
		return
	}
	ep := n.endpoint(eid)
	if ep == nil {
		logrus.Debugf("Error pushing local endpoint event for ep %s / %s", nid, eid)
		return
	}

	if !d.isSerfAlive() {
		return
	}
	d.notifyCh <- ovNotify{
		action: action,
		nw:     n,
		ep:     ep,
	}
}

// DiscoverNew is a notification for a new discovery event, such as a new node joining a cluster
func (d *driver) DiscoverNew(dType discoverapi.DiscoveryType, data interface{}) error {

	var err error
	switch dType {
	case discoverapi.NodeDiscovery:
		nodeData, ok := data.(discoverapi.NodeDiscoveryData)
		if !ok || nodeData.Address == "" {
			return fmt.Errorf("invalid discovery data")
		}
		d.nodeJoin(nodeData.Address, nodeData.BindAddress, nodeData.Self)
	case discoverapi.DatastoreConfig:
		if d.store != nil {
			return types.ForbiddenErrorf("cannot accept datastore configuration: Overlay driver has a datastore configured already")
		}
		dsc, ok := data.(discoverapi.DatastoreConfigData)
		if !ok {
			return types.InternalErrorf("incorrect data in datastore configuration: %v", data)
		}
		d.store, err = datastore.NewDataStoreFromConfig(dsc)
		if err != nil {
			return types.InternalErrorf("failed to initialize data store: %v", err)
		}
	default:
	}
	return nil
}

// DiscoverDelete is a notification for a discovery delete event, such as a node leaving a cluster
func (d *driver) DiscoverDelete(dType discoverapi.DiscoveryType, data interface{}) error {
	return nil
}

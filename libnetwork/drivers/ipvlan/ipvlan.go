package ipvlan

import (
	"net"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/discoverapi"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/osl"
)

const (
	vethLen             = 7
	containerVethPrefix = "eth"
	vethPrefix          = "veth"
	ipvlanType          = "ipvlan"     // driver type name
	modeL2              = "l2"         // ipvlan mode l2 is the default
	modeL3              = "l3"         // ipvlan L3 mode
	hostIfaceOpt        = "host_iface" // host interface -o host_iface
	modeOpt             = "_mode"      // ipvlan mode ux opt suffix
)

var driverModeOpt = ipvlanType + modeOpt // mode --option ipvlan_mode

type endpointTable map[string]*endpoint

type networkTable map[string]*network

type driver struct {
	networks networkTable
	sync.Once
	sync.Mutex
	store datastore.DataStore
}

type endpoint struct {
	id      string
	mac     net.HardwareAddr
	addr    *net.IPNet
	addrv6  *net.IPNet
	srcName string
}

type network struct {
	id        string
	sbox      osl.Sandbox
	endpoints endpointTable
	driver    *driver
	config    *configuration
	sync.Mutex
}

// Init initializes and registers the libnetwork ipvlan driver
func Init(dc driverapi.DriverCallback, config map[string]interface{}) error {
	if err := kernelSupport(ipvlanType); err != nil {
		logrus.Warnf("encountered issues loading the ipvlan kernel module: %v", err)
	}
	c := driverapi.Capability{
		DataScope: datastore.LocalScope,
	}
	d := &driver{
		networks: networkTable{},
	}
	d.initStore(config)
	return dc.RegisterDriver(ipvlanType, d, c)
}

func (d *driver) EndpointOperInfo(nid, eid string) (map[string]interface{}, error) {
	return make(map[string]interface{}, 0), nil
}

func (d *driver) Type() string {
	return ipvlanType
}

// DiscoverNew is a notification for a new discovery event.
func (d *driver) DiscoverNew(dType discoverapi.DiscoveryType, data interface{}) error {
	return nil
}

// DiscoverDelete is a notification for a discovery delete event.
func (d *driver) DiscoverDelete(dType discoverapi.DiscoveryType, data interface{}) error {
	return nil
}

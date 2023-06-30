package overlay

//go:generate protoc -I=. -I=../../../../vendor/ --gogo_out=import_path=github.com/docker/docker/libnetwork/drivers/overlay:. overlay.proto

import (
	"context"
	"encoding/json"
	"net"
	"sync"

	"github.com/Microsoft/hcsshim"
	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/discoverapi"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/types"
)

const (
	networkType = "overlay"
)

type driver struct {
	networks networkTable
	sync.Mutex
}

// Register registers a new instance of the overlay driver.
func Register(r driverapi.Registerer, _ map[string]interface{}) error {
	c := driverapi.Capability{
		DataScope:         datastore.GlobalScope,
		ConnectivityScope: datastore.GlobalScope,
	}

	d := &driver{
		networks: networkTable{},
	}

	d.restoreHNSNetworks()

	return r.RegisterDriver(networkType, d, c)
}

func (d *driver) restoreHNSNetworks() error {
	log.G(context.TODO()).Infof("Restoring existing overlay networks from HNS into docker")

	hnsresponse, err := hcsshim.HNSListNetworkRequest("GET", "", "")
	if err != nil {
		return err
	}

	for _, v := range hnsresponse {
		if v.Type != networkType {
			continue
		}

		log.G(context.TODO()).Infof("Restoring overlay network: %s", v.Name)
		n := d.convertToOverlayNetwork(&v)
		d.addNetwork(n)

		//
		// We assume that any network will be recreated on daemon restart
		// and therefore don't restore hns endpoints for now
		//
		// n.restoreNetworkEndpoints()
	}

	return nil
}

func (d *driver) convertToOverlayNetwork(v *hcsshim.HNSNetwork) *network {
	n := &network{
		id:              v.Name,
		hnsID:           v.Id,
		driver:          d,
		endpoints:       endpointTable{},
		subnets:         []*subnet{},
		providerAddress: v.ManagementIP,
	}

	for _, hnsSubnet := range v.Subnets {
		vsidPolicy := &hcsshim.VsidPolicy{}
		for _, policy := range hnsSubnet.Policies {
			if err := json.Unmarshal([]byte(policy), &vsidPolicy); err == nil && vsidPolicy.Type == "VSID" {
				break
			}
		}

		gwIP := net.ParseIP(hnsSubnet.GatewayAddress)
		localsubnet := &subnet{
			vni:  uint32(vsidPolicy.VSID),
			gwIP: &gwIP,
		}

		_, subnetIP, err := net.ParseCIDR(hnsSubnet.AddressPrefix)
		if err != nil {
			log.G(context.TODO()).Errorf("Error parsing subnet address %s ", hnsSubnet.AddressPrefix)
			continue
		}

		localsubnet.subnetIP = subnetIP

		n.subnets = append(n.subnets, localsubnet)
	}

	return n
}

func (d *driver) Type() string {
	return networkType
}

func (d *driver) IsBuiltIn() bool {
	return true
}

// DiscoverNew is a notification for a new discovery event, such as a new node joining a cluster
func (d *driver) DiscoverNew(dType discoverapi.DiscoveryType, data interface{}) error {
	return types.NotImplementedErrorf("not implemented")
}

// DiscoverDelete is a notification for a discovery delete event, such as a node leaving a cluster
func (d *driver) DiscoverDelete(dType discoverapi.DiscoveryType, data interface{}) error {
	return types.NotImplementedErrorf("not implemented")
}

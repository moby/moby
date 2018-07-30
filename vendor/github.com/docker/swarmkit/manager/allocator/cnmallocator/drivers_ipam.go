package cnmallocator

import (
	"net"
	"strconv"
	"strings"

	"github.com/docker/libnetwork/drvregistry"
	"github.com/docker/libnetwork/ipamapi"
	builtinIpam "github.com/docker/libnetwork/ipams/builtin"
	nullIpam "github.com/docker/libnetwork/ipams/null"
	remoteIpam "github.com/docker/libnetwork/ipams/remote"
	"github.com/docker/libnetwork/ipamutils"
	"github.com/sirupsen/logrus"
)

func initIPAMDrivers(r *drvregistry.DrvRegistry, defaultAddrPool []*net.IPNet, subnetSize int) error {
	var addressPool []*ipamutils.NetworkToSplit
	var str strings.Builder
	str.WriteString("Subnetlist - ")
	// Extract defaultAddrPool param info and construct ipamutils.NetworkToSplit
	// from the info. We will be using it to call Libnetwork API
	// We also need to log new address pool info whenever swarm init
	// happens with default address pool option
	if defaultAddrPool != nil {
		for _, p := range defaultAddrPool {
			addressPool = append(addressPool, &ipamutils.NetworkToSplit{
				Base: p.String(),
				Size: subnetSize,
			})
			str.WriteString(p.String() + ",")
		}
		str.WriteString(": Size ")
		str.WriteString(strconv.Itoa(subnetSize))
	}
	if err := ipamutils.ConfigGlobalScopeDefaultNetworks(addressPool); err != nil {
		return err
	}
	if addressPool != nil {
		logrus.Infof("Swarm initialized global default address pool to: " + str.String())
	}

	for _, fn := range [](func(ipamapi.Callback, interface{}, interface{}) error){
		builtinIpam.Init,
		remoteIpam.Init,
		nullIpam.Init,
	} {
		if err := fn(r, nil, nil); err != nil {
			return err
		}
	}

	return nil
}

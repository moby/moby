//go:build windows

package libnetwork

import (
	"context"
	"runtime"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork/drivers/windows"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipams/windowsipam"
)

func executeInCompartment(compartmentID uint32, x func()) {
	runtime.LockOSThread()

	if err := hcsshim.SetCurrentThreadCompartmentId(compartmentID); err != nil {
		log.G(context.TODO()).Error(err)
	}
	defer func() {
		hcsshim.SetCurrentThreadCompartmentId(0)
		runtime.UnlockOSThread()
	}()

	x()
}

func (n *Network) startResolver() {
	if n.networkType == "ics" {
		return
	}
	n.resolverOnce.Do(func() {
		log.G(context.TODO()).Debugf("Launching DNS server for network %q", n.Name())
		hnsid := n.DriverOptions()[windows.HNSID]
		if hnsid == "" {
			return
		}

		hnsresponse, err := hcsshim.HNSNetworkRequest("GET", hnsid, "")
		if err != nil {
			log.G(context.TODO()).Errorf("Resolver Setup/Start failed for container %s, %q", n.Name(), err)
			return
		}

		for _, subnet := range hnsresponse.Subnets {
			if subnet.GatewayAddress != "" {
				for i := 0; i < 3; i++ {
					resolver := NewResolver(subnet.GatewayAddress, false, n)
					log.G(context.TODO()).Debugf("Binding a resolver on network %s gateway %s", n.Name(), subnet.GatewayAddress)
					executeInCompartment(hnsresponse.DNSServerCompartment, resolver.SetupFunc(53))

					if err = resolver.Start(); err != nil {
						log.G(context.TODO()).Errorf("Resolver Setup/Start failed for container %s, %q", n.Name(), err)
						time.Sleep(1 * time.Second)
					} else {
						log.G(context.TODO()).Debugf("Resolver bound successfully for network %s", n.Name())
						n.resolver = append(n.resolver, resolver)
						break
					}
				}
			}
		}
	})
}

func defaultIpamForNetworkType(networkType string) string {
	if windows.IsBuiltinLocalDriver(networkType) {
		return windowsipam.DefaultIPAM
	}
	return ipamapi.DefaultIPAM
}

package main

import (
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/pkg/options"
	"github.com/docker/libnetwork/sandbox"
)

func main() {
	// Create a new controller instance
	controller := libnetwork.New()

	option := options.Generic{}
	driver, err := controller.NewNetworkDriver("simplebridge", option)
	if err != nil {
		return
	}

	netOptions := options.Generic{}
	// Create a network for containers to join.
	network, err := controller.NewNetwork(driver, "network1", netOptions)
	if err != nil {
		return
	}

	// For a new container: create a sandbox instance (providing a unique key).
	// For linux it is a filesystem path
	networkPath := "/var/lib/docker/.../4d23e"
	networkNamespace, err := sandbox.NewSandbox(networkPath)
	if err != nil {
		return
	}

	// For each new container: allocate IP and interfaces. The returned network
	// settings will be used for container infos (inspect and such), as well as
	// iptables rules for port publishing.
	_, sinfo, err := network.CreateEndpoint("Endpoint1", networkNamespace.Key(), "")
	if err != nil {
		return
	}

	// Add interfaces to the namespace.
	for _, iface := range sinfo.Interfaces {
		if err := networkNamespace.AddInterface(iface); err != nil {
			return
		}
	}

	// Set the gateway IP
	if err := networkNamespace.SetGateway(sinfo.Gateway); err != nil {
		return
	}
}

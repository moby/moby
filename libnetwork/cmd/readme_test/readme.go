package main

import (
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/pkg/options"
)

func main() {
	// Create a new controller instance
	controller := libnetwork.New()

	// Select and configure the network driver
	networkType := "bridge"
	option := options.Generic{}
	err := controller.ConfigureNetworkDriver(networkType, option)
	if err != nil {
		return
	}

	netOptions := options.Generic{}
	// Create a network for containers to join.
	network, err := controller.NewNetwork(networkType, "network1", netOptions)
	if err != nil {
		return
	}

	// For each new container: allocate IP and interfaces. The returned network
	// settings will be used for container infos (inspect and such), as well as
	// iptables rules for port publishing. This info is contained or accessible
	// from the returned endpoint.
	ep, err := network.CreateEndpoint("Endpoint1", nil)
	if err != nil {
		return
	}

	// A container can join the endpoint by providing the container ID to the join
	// api which returns the sandbox key which can be used to access the sandbox
	// created for the container during join.
	_, err = ep.Join("container1")
	if err != nil {
		return
	}
}

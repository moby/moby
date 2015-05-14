package main

import (
	"fmt"

	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/options"
)

func main() {
	// Create a new controller instance
	controller, err := libnetwork.New()
	if err != nil {
		return
	}

	// Select and configure the network driver
	networkType := "bridge"

	driverOptions := options.Generic{}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = driverOptions
	err = controller.ConfigureNetworkDriver(networkType, genericOption)
	if err != nil {
		return
	}

	// Create a network for containers to join.
	// NewNetwork accepts Variadic optional arguments that libnetwork and Drivers can make of
	network, err := controller.NewNetwork(networkType, "network1")
	if err != nil {
		return
	}

	// For each new container: allocate IP and interfaces. The returned network
	// settings will be used for container infos (inspect and such), as well as
	// iptables rules for port publishing. This info is contained or accessible
	// from the returned endpoint.
	ep, err := network.CreateEndpoint("Endpoint1")
	if err != nil {
		return
	}

	// A container can join the endpoint by providing the container ID to the join
	// api which returns the sandbox key which can be used to access the sandbox
	// created for the container during join.
	// Join acceps Variadic arguments which will be made use of by libnetwork and Drivers
	_, err = ep.Join("container1",
		libnetwork.JoinOptionHostname("test"),
		libnetwork.JoinOptionDomainname("docker.io"))
	if err != nil {
		return
	}

	// libentwork client can check the endpoint's operational data via the Info() API
	epInfo, err := ep.DriverInfo()
	mapData, ok := epInfo[netlabel.PortMap]
	if ok {
		portMapping, ok := mapData.([]netutils.PortBinding)
		if ok {
			fmt.Printf("Current port mapping for endpoint %s: %v", ep.Name(), portMapping)
		}
	}
}

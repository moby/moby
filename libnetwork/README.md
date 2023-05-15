# libnetwork - networking for containers

Libnetwork provides a native Go implementation for connecting containers

The goal of libnetwork is to deliver a robust Container Network Model that provides a consistent programming interface and the required network abstractions for applications.

#### Design
Please refer to the [design](docs/design.md) for more information.

#### Using libnetwork

There are many networking solutions available to suit a broad range of use-cases. libnetwork uses a driver / plugin model to support all of these solutions while abstracting the complexity of the driver implementations by exposing a simple and consistent Network Model to users.


```go
package main

import (
	"fmt"
	"log"

	"github.com/docker/docker/libnetwork"
	"github.com/docker/docker/libnetwork/config"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/options"
)

func main() {
	// Select and configure the network driver
	networkType := "bridge"

	// Create a new controller instance
	driverOptions := options.Generic{}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = driverOptions
	controller, err := libnetwork.New(config.OptionDriverConfig(networkType, genericOption))
	if err != nil {
		log.Fatalf("libnetwork.New: %s", err)
	}

	// Create a network for containers to join.
	// NewNetwork accepts Variadic optional arguments that libnetwork and Drivers can use.
	network, err := controller.NewNetwork(networkType, "network1", "")
	if err != nil {
		log.Fatalf("controller.NewNetwork: %s", err)
	}

	// For each new container: allocate IP and interfaces. The returned network
	// settings will be used for container infos (inspect and such), as well as
	// iptables rules for port publishing. This info is contained or accessible
	// from the returned endpoint.
	ep, err := network.CreateEndpoint("Endpoint1")
	if err != nil {
		log.Fatalf("network.CreateEndpoint: %s", err)
	}

	// Create the sandbox for the container.
	// NewSandbox accepts Variadic optional arguments which libnetwork can use.
	sbx, err := controller.NewSandbox("container1",
		libnetwork.OptionHostname("test"),
		libnetwork.OptionDomainname("example.com"))
	if err != nil {
		log.Fatalf("controller.NewSandbox: %s", err)
	}

	// A sandbox can join the endpoint via the join api.
	err = ep.Join(sbx)
	if err != nil {
		log.Fatalf("ep.Join: %s", err)
	}

	// libnetwork client can check the endpoint's operational data via the Info() API
	epInfo, err := ep.DriverInfo()
	if err != nil {
		log.Fatalf("ep.DriverInfo: %s", err)
	}

	macAddress, ok := epInfo[netlabel.MacAddress]
	if !ok {
		log.Fatal("failed to get mac address from endpoint info")
	}

	fmt.Printf("Joined endpoint %s (%s) to sandbox %s (%s)\n", ep.Name(), macAddress, sbx.ContainerID(), sbx.Key())
}
```

## Contributing

Want to hack on libnetwork? [Docker's contributions guidelines](https://github.com/docker/docker/blob/master/CONTRIBUTING.md) apply.

## Copyright and license
Code and documentation copyright 2015 Docker, inc. Code released under the Apache 2.0 license. Docs released under Creative commons.

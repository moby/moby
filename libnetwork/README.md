# libnetwork - networking for containers

[![Circle CI](https://circleci.com/gh/docker/libnetwork/tree/master.svg?style=svg)](https://circleci.com/gh/docker/libnetwork/tree/master) [![Coverage Status](https://coveralls.io/repos/docker/libnetwork/badge.svg)](https://coveralls.io/r/docker/libnetwork) [![GoDoc](https://godoc.org/github.com/docker/libnetwork?status.svg)](https://godoc.org/github.com/docker/libnetwork)

Libnetwork provides a native Go implementation for connecting containers

The goal of libnetwork is to deliver a robust Container Network Model that provides a consistent programming interface and the required network abstractions for applications.

**NOTE**: libnetwork project is under heavy development and is not ready for general use.

#### Design
Please refer to the [design](docs/design.md) for more information.

#### Using libnetwork

There are many networking solutions available to suit a broad range of use-cases. libnetwork uses a driver / plugin model to support all of these solutions while abstracting the complexity of the driver implementations by exposing a simple and consistent Network Model to users.


```go
        // Create a new controller instance
        controller := libnetwork.New()

        // Select and configure the network driver
        networkType := "bridge"

        driverOptions := options.Generic{}
        genericOption := make(map[string]interface{})
        genericOption[netlabel.GenericData] = driverOptions
        err := controller.ConfigureNetworkDriver(networkType, genericOption)
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
        // api.
        // Join acceps Variadic arguments which will be made use of by libnetwork and Drivers
        err = ep.Join("container1",
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

```
#### Current Status
Please watch this space for updates on the progress.

Currently libnetwork is nothing more than an attempt to modularize the Docker platform's networking subsystem by moving it into libnetwork as a library.

## Future
Please refer to [roadmap](ROADMAP.md) for more information.

## Contributing

Want to hack on libnetwork? [Docker's contributions guidelines](https://github.com/docker/docker/blob/master/CONTRIBUTING.md) apply.

## Copyright and license
Code and documentation copyright 2015 Docker, inc. Code released under the Apache 2.0 license. Docs released under Creative commons.


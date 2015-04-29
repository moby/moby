# libnetwork - networking for containers

[![Circle CI](https://circleci.com/gh/docker/libnetwork/tree/master.svg?style=svg)](https://circleci.com/gh/docker/libnetwork/tree/master) [![Coverage Status](https://coveralls.io/repos/docker/libnetwork/badge.svg)](https://coveralls.io/r/docker/libnetwork) [![GoDoc](https://godoc.org/github.com/docker/libnetwork?status.svg)](https://godoc.org/github.com/docker/libnetwork)

Libnetwork provides a native Go implementation for connecting containers

The goal of libnetwork is to deliver a robust Container Network Model that provides a consistent programming interface and the required network abstractions for applications.

**NOTE**: libnetwork project is under heavy development and is not ready for general use.

#### Current Status
Please watch this space for updates on the progress.

Currently libnetwork is nothing more than an attempt to modularize the Docker platform's networking subsystem by moving it into libnetwork as a library.

Please refer to the [roadmap](ROADMAP.md) for more information.

#### Using libnetwork

There are many networking solutions available to suit a broad range of use-cases. libnetwork uses a driver / plugin model to support all of these solutions while abstracting the complexity of the driver implementations by exposing a simple and consistent Network Model to users.


```go
    // Create a new controller instance
    controller := libnetwork.New()

    // This option is only needed for in-tree drivers. Plugins(in future) will get
    // their options through plugin infrastructure.
    option := options.Generic{}
    err := controller.NewNetworkDriver("bridge", option)
    if err != nil {
        return
    }

    netOptions := options.Generic{}
    // Create a network for containers to join.
    network, err := controller.NewNetwork("bridge", "network1", netOptions)
    if err != nil {
    	return
    }

    // For each new container: allocate IP and interfaces. The returned network
    // settings will be used for container infos (inspect and such), as well as
    // iptables rules for port publishing.
    ep, err := network.CreateEndpoint("Endpoint1", nil)
    if err != nil {
	    return
    }
```

## Future
See the [roadmap](ROADMAP.md).

## Contributing

Want to hack on libnetwork? [Docker's contributions guidelines](https://github.com/docker/docker/blob/master/CONTRIBUTING.md) apply.

## Copyright and license
Code and documentation copyright 2015 Docker, inc. Code released under the Apache 2.0 license. Docs released under Creative commons.


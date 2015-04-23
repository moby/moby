Design
======

The main goals of libnetwork are highlighted in the [roadmap](../ROADMAP.md).
This document describes how libnetwork has been designed in order to acheive this.
Requirements for individual releases can be found on the [Project Page](https://github.com/docker/libnetwork/wiki)

## Legacy Docker Networking

Prior to libnetwork a container's networking was handled in both Docker Engine and libcontainer.
Docker Engine was responsible for providing the configuration of the container's networking stack.
Libcontainer would then use this information to create the necessary networking devices and move them in to a network namespace.
This namespace would then be used when the container is started.

## The Container Network Model

Libnetwork implements Container Network Model (CNM) which formalizes the steps required to provide networking for containers while providing an abstraction that can be used to support multiple network drivers. The CNM is built on 3 main components.

**Sandbox**

A Sandbox contains the configuration of a container's network stack.
This includes management of the container's interfaces, routing table and DNS settings. 
An implementation of a Sandbox could be a Linux Network Namespace, a FreeBSD Jail or other similar concept.
A Sandbox may contain *many* endpoints from *multiple* networks

**Endpoint**

An Endpoint joins a Sandbox to a Network.
An implementation of an Endpoint could be a `veth` pair, an Open vSwitch internal port or similar.
An Endpoint can belong to *only one* network but may only belong to *one* Sandbox

**Network**

A Network is a group of Endpoints that are able to communicate with each-other directly.
An implementation of a Network could be a Linux bridge, a VLAN etc...
Networks consist of *many* endpoints

## API

Consumers of the CNM, like Docker for example, interact through the following APIs

The `NetworkController` object is created to manage the allocation of Networks and the binding of these Networks to a specific Driver
Once a Network is created, `network.CreateEndpoint` can be called to create a new Endpoint in a given network.
When an Endpoint exists, it can be joined to a Sandbox using `endpoint.Join(id)`. If no Sandbox exists, one will be created, but if the Sandbox already exists, the endpoint will be added there.
The result of the Join operation is a Sandbox Key which identifies the Sandbox to the Operating System (e.g a path)
This Key can be passed to the container runtime so the Sandbox is used when the container is started.

When the container is stopped, `endpoint.Leave` will be called on each endpoint within the Sandbox
Finally once, endpoint.

## Component Lifecycle

### Sandbox Lifecycle

The Sandbox is created during the first `endpoint.Join` and deleted when `endpoint.Leave` is called on the last endpoint.
<TODO @mrjana or @mavenugo to more details>

### Endpoint Lifecycle 

The Endpoint is created on `network.CreateEndpoint` and removed on `endpoint.Delete`
<TODO @mrjana or @mavenugo to add details on when this is called>

### Network Lifecycle

Networks are created when the CNM API call is invoked and are not cleaned up until an corresponding delete API call is made.

## Implementation

Networks and Endpoints are mostly implemented in drivers. For more information on these details, please see [the drivers section](#Drivers)

## Sandbox

Libnetwork provides an implementation of a Sandbox for Linux.
This creates a Network Namespace for each sandbox which is uniquely identified by a path on the host filesystem.
Netlink calls are used to move interfaces from the global namespace to the Sandbox namespace.
Netlink is also used to manage the routing table in the namespace.

# Drivers

## API

The Driver API allows libnetwork to defer to a driver to provide Networking services.

For Networks, drivers are notified on Create and Delete events

For Endpoints, drivers are also notified on Create and Delete events

## Implementations

Libnetwork includes the following drivers:

- null
- bridge
- overlay
- remote

### Null

The null driver is a `noop` implementation of the driver API, used only in cases where no networking is desired.

### Bridge

The `bridge` driver provides a Linux-specific bridging implementation based on the Linux Bridge.
For more details, please [see the Bridge Driver documentation](bridge.md)

### Overlay

The `overlay` driver implements networking that can span multiple hosts using overlay network encapsulations such as VXLAN.
For more details on its design, please see the [Overlay Driver Design](overlay.md)

### Remote

The `remote` driver, provides a means of supporting drivers over a remote transport.
This allows a driver to be written in a language of your choice.
For further details, please see the [Remote Driver Design](remote.md)


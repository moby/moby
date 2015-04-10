# libnetwork: what's next?

This document is a high-level overview of where we want to take libnetwork next.
It is a curated selection of planned improvements which are either important, difficult, or both.

For a more complete view of planned and requested improvements, see [the Github issues](https://github.com/docker/libnetwork/issues).

To suggest changes to the roadmap, including additions, please write the change as if it were already in effect, and make a pull request.

## Container Network Model (CNM)

#### Concepts

1. Sandbox: An isolated environment. This is more or less a standard docker container.
2. Endpoint: An addressable endpoint used for communication over a specific network. Endpoints join exactly one network and are expected to create a method of network communication for a container. Endpoints are garbage collected when they no longer belong to any Sandboxes. Example : veth pair
3. Network: A collection of endpoints that are able to communicate to each other. These networks are intended to be isolated from each other and do not cross communicate. Networks house endpoints which can communicate with each other. 

#### axioms
The container network model is a few axioms about how libnetwork wishes to supply
interoperation between networks and containers.

1. All containers on a specific network can communicate with each other freely.
2. Multiple networks are the way to segment traffic between containers and should be supported by all drivers.
3. Multiple endpoints per container are the way to join a container to multiple networks.
4. An endpoint is added to a sandbox to provide it with network connectivity.

## Bridge Driver using CNM
Existing native networking functionality of Docker will be implemented as a Bridge Driver using the above CNM.  In order to prove the effectiveness of the Bridge Driver, we will make necessary  modifications to Docker Daemon and LibContainer to replace the existing networking functionality with libnetwork & Bridge Driver.

## Plugin support
The Driver model provides a modular way to allow different networking solutions to be used as the backend. But they are static in nature. 
Plugins solves that problem by supporting dynamic pluggable networking backend for libnetwork.
There are other community efforts in developing Plugin support on the Docker platform.libnetwork project will also make use of it when available.


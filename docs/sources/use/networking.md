page_title: Configure Networking
page_description: Docker networking
page_keywords: network, networking, bridge, docker, documentation

# Configure Networking

## Introduction

Docker uses Linux bridge capabilities to provide network connectivity to
containers. The `docker0` bridge interface is
managed by Docker for this purpose. When the Docker daemon starts it :

- creates the `docker0` bridge if not present
- searches for an IP address range which doesn’t overlap with an existing route
- picks an IP in the selected range
- assigns this IP to the `docker0` bridge

<!-- -->

    # List host bridges
    $ sudo brctl show
    bridge      name    bridge id               STP enabled     interfaces
    docker0             8000.000000000000       no

    # Show docker0 IP address
    $ sudo ifconfig docker0
    docker0   Link encap:Ethernet  HWaddr xx:xx:xx:xx:xx:xx
         inet addr:172.17.42.1  Bcast:0.0.0.0  Mask:255.255.0.0

At runtime, a [*specific kind of virtual interface*](#vethxxxx-device)
is given to each container which is then bonded to the `docker0` bridge.
Each container also receives a dedicated IP address from the same range
as `docker0`. The `docker0` IP address is used as the default gateway
for the container.

    # Run a container
    $ sudo docker run -t -i -d base /bin/bash
    52f811c5d3d69edddefc75aff5a4525fc8ba8bcfa1818132f9dc7d4f7c7e78b4

    $ sudo brctl show
    bridge      name    bridge id               STP enabled     interfaces
    docker0             8000.fef213db5a66       no              vethQCDY1N

Above, `docker0` acts as a bridge for the `vethQCDY1N` interface which
is dedicated to the 52f811c5d3d6 container.

## How to use a specific IP address range

Docker will try hard to find an IP range that is not used by the host.
Even though it works for most cases, it’s not bullet-proof and sometimes
you need to have more control over the IP addressing scheme.

For this purpose, Docker allows you to manage the `docker0`
bridge or your own one using the `-b=<bridgename>`
parameter.

In this scenario:

-   ensure Docker is stopped
-   create your own bridge (`bridge0` for example)
-   assign a specific IP to this bridge
-   start Docker with the `-b=bridge0` parameter

<!-- -->

    # Stop Docker
    $ sudo service docker stop

    # Clean docker0 bridge and
    # add your very own bridge0
    $ sudo ifconfig docker0 down
    $ sudo brctl addbr bridge0
    $ sudo ifconfig bridge0 192.168.227.1 netmask 255.255.255.0

    # Edit your Docker startup file
    $ echo "DOCKER_OPTS=\"-b=bridge0\"" >> /etc/default/docker

    # Start Docker
    $ sudo service docker start

    # Ensure bridge0 IP is not changed by Docker
    $ sudo ifconfig bridge0
    bridge0   Link encap:Ethernet  HWaddr xx:xx:xx:xx:xx:xx
              inet addr:192.168.227.1  Bcast:192.168.227.255  Mask:255.255.255.0

    # Run a container
    $ docker run -i -t base /bin/bash

    # Container IP in the 192.168.227/24 range
    root@261c272cd7d5:/# ifconfig eth0
    eth0      Link encap:Ethernet  HWaddr xx:xx:xx:xx:xx:xx
              inet addr:192.168.227.5  Bcast:192.168.227.255  Mask:255.255.255.0

    # bridge0 IP as the default gateway
    root@261c272cd7d5:/# route -n
    Kernel IP routing table
    Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
    0.0.0.0         192.168.227.1   0.0.0.0         UG    0      0        0 eth0
    192.168.227.0   0.0.0.0         255.255.255.0   U     0      0        0 eth0

    # hits CTRL+P then CTRL+Q to detach

    # Display bridge info
    $ sudo brctl show
    bridge      name    bridge id               STP enabled     interfaces
    bridge0             8000.fe7c2e0faebd       no              vethAQI2QT

## Container intercommunication

The value of the Docker daemon’s `icc` parameter
determines whether containers can communicate with each other over the
bridge network.

-   The default, `-icc=true` allows containers to
    communicate with each other.
-   `-icc=false` means containers are isolated from
    each other.

Docker uses `iptables` under the hood to either
accept or drop communication between containers.

## What is the vethXXXX device?

Well. Things get complicated here.

The `vethXXXX` interface is the host side of a
point-to-point link between the host and the corresponding container;
the other side of the link is the container’s `eth0`
interface. This pair (host `vethXXX` and container
`eth0`) are connected like a tube. Everything that
comes in one side will come out the other side.

All the plumbing is delegated to Linux network capabilities (check the
ip link command) and the namespaces infrastructure.

## I want more

Jérôme Petazzoni has create `pipework` to connect
together containers in arbitrarily complex scenarios :
[https://github.com/jpetazzo/pipework](https://github.com/jpetazzo/pipework)

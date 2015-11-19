<!--[metadata]>
+++
title = "Customize the docker0 bridge"
description = "Customizing docker0"
keywords = ["docker, bridge, docker0, network"]
[menu.main]
parent = "smn_networking_def"
+++
<![end-metadata]-->

# Customize the docker0 bridge

The information in this section explains how to customize the Docker default bridge. This is a `bridge` network named `bridge` created automatically when you install Docker.  

**Note**: The [Docker networks feature](../dockernetworks.md) allows you to create user-defined networks in addition to the default bridge network.

By default, the Docker server creates and configures the host system's `docker0` interface as an _Ethernet bridge_ inside the Linux kernel that can pass packets back and forth between other physical or virtual network interfaces so that they behave as a single Ethernet network.

Docker configures `docker0` with an IP address, netmask and IP allocation range. The host machine can both receive and send packets to containers connected to the bridge, and gives it an MTU -- the _maximum transmission unit_ or largest packet length that the interface will allow -- of either 1,500 bytes or else a more specific value copied from the Docker host's interface that supports its default route.  These options are configurable at server startup:
- `--bip=CIDR` -- supply a specific IP address and netmask for the `docker0` bridge, using standard CIDR notation like `192.168.1.5/24`.

- `--fixed-cidr=CIDR` -- restrict the IP range from the `docker0` subnet, using the standard CIDR notation like `172.167.1.0/28`. This range must be an IPv4 range for fixed IPs (ex: 10.20.0.0/16) and must be a subset of the bridge IP range (`docker0` or set using `--bridge`). For example with `--fixed-cidr=192.168.1.0/25`, IPs for your containers will be chosen from the first half of `192.168.1.0/24` subnet.

- `--mtu=BYTES` -- override the maximum packet length on `docker0`.

Once you have one or more containers up and running, you can confirm that Docker has properly connected them to the `docker0` bridge by running the `brctl` command on the host machine and looking at the `interfaces` column of the output.  Here is a host with two different containers connected:

```
# Display bridge info

$ sudo brctl show
bridge name     bridge id               STP enabled     interfaces
docker0         8000.3a1d7362b4ee       no              veth65f9
                                                        vethdda6
```

If the `brctl` command is not installed on your Docker host, then on Ubuntu you should be able to run `sudo apt-get install bridge-utils` to install it.

Finally, the `docker0` Ethernet bridge settings are used every time you create a new container.  Docker selects a free IP address from the range available on the bridge each time you `docker run` a new container, and configures the container's `eth0` interface with that IP address and the bridge's netmask.  The Docker host's own IP address on the bridge is used as the default gateway by which each container reaches the rest of the Internet.

```
# The network, as seen from a container

$ docker run -i -t --rm base /bin/bash

$$ ip addr show eth0
24: eth0: <BROADCAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP group default qlen 1000
    link/ether 32:6f:e0:35:57:91 brd ff:ff:ff:ff:ff:ff
    inet 172.17.0.3/16 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::306f:e0ff:fe35:5791/64 scope link
       valid_lft forever preferred_lft forever

$$ ip route
default via 172.17.42.1 dev eth0
172.17.0.0/16 dev eth0  proto kernel  scope link  src 172.17.0.3

$$ exit
```

Remember that the Docker host will not be willing to forward container packets out on to the Internet unless its `ip_forward` system setting is `1` -- see the section above on [Communication between containers](#between-containers) for details.

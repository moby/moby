<!--[metadata]>
+++
title = "Get started with macvlan network driver"
description = "Use macvlan for container networking"
keywords = ["Examples, Usage, network, docker, documentation, user guide, macvlan, cluster"]
[menu.main]
parent = "smn_networking"
weight=-3
+++
<![end-metadata]-->

# Macvlan Network Driver

### Getting Started

The Macvlan driver is in order to make Docker users use cases and vet the implementation to ensure a hardened, production ready driver. Libnetwork now gives users total control over both IPv4 and IPv6 addressing. The VLAN drivers build on top of that in giving operators complete control of layer 2 VLAN tagging for users interested in underlay network integration. For overlay deployments that abstract away physical constraints see the [multi-host overlay ](https://docs.docker.com/engine/userguide/networking/get-started-overlay/) driver.

Macvlan is a new twist on the tried and true network virtualization technique. The Linux implementations are extremely lightweight because rather than using the traditional Linux bridge for isolation, they are simply associated to a Linux Ethernet interface or sub-interface to enforce separation between networks and connectivity to the physical network.

Macvlan offers a number of unique features and plenty of room for further innovations with the various modes. Two high level advantages of these approaches are, the positive performance implications of bypassing the Linux bridge and the simplicity of having less moving parts. Removing the bridge that traditionally resides in between the Docker host NIC and container interface leaves a very simple setup consisting of container interfaces, attached directly to the Docker host interface. This result is easy access for external facing services as there is no port mappings in these scenarios.

### Pre-Requisites

- The examples on this page are all single host and setup using Docker 1.12.0+

- All of the examples can be performed on a single host running Docker. Any examples using a sub-interface like `eth0.10` can be replaced with `eth0` or any other valid parent interface on the Docker host. Sub-interfaces with a `.` are created on the fly. `-o parent` interfaces can also be left out of the `docker network create` all together and the driver will create a `dummy` interface that will enable local host connectivity to perform the examples.

- Kernel requirements:
 
 - To check your current kernel version, use `uname -r` to display your kernel version
 - Macvlan Linux kernel v3.9–3.19 and 4.0+

### MacVlan Bridge Mode Example Usage

Macvlan Bridge mode has a unique MAC address per container used to track MAC to port mappings by the Docker host.

- Macvlan driver networks are attached to a parent Docker host interface. Examples are a physical interface such as `eth0`, a sub-interface for 802.1q VLAN tagging like `eth0.10` (`.10` representing VLAN `10`) or even bonded host adaptors which bundle two Ethernet interfaces into a single logical interface.

- The specified gateway is external to the host provided by the network infrastructure. 

- Each Macvlan Bridge mode Docker network is isolated from one another and there can be only one network attached to a parent interface at a time. There is a theoretical limit of 4,094 sub-interfaces per host adaptor that a Docker network could be attached to.

- Any container inside the same subnet can talk to any other container in the same network without a  gateway in `macvlan bridge`.

- The same `docker network` commands apply to the vlan drivers. 

- In Macvlan mode, containers on separate networks cannot reach one another without an external process routing between the two networks/subnets. This also applies to multiple subnets within the same `docker network

In the following example, `eth0` on the docker host has an IP on the `172.16.86.0/24` network and a default gateway of `172.16.86.1`. The gateway is an external router with an address of `172.16.86.1`. An IP address is not required on the Docker host interface `eth0` in `bridge` mode, it merely needs to be on the proper upstream network to get forwarded by a network switch or network router.

![Simple Macvlan Bridge Mode Example](images/macvlan_bridge_simple.png)

**Note** For Macvlan bridge mode the subnet values need to match the NIC's interface of the Docker host. For example, Use the same subnet and gateway of the Docker host ethernet interface that is specified by the `-o parent=` option.

- The parent interface used in this example is `eth0` and it is on the subnet `172.16.86.0/24`. The containers in the `docker network` will also need to be on this same subnet as the parent `-o parent=`. The gateway is an external router on the network, not any ip masquerading or any other local proxy.

- The driver is specified with `-d driver_name` option. In this case `-d macvlan`

- The parent interface `-o parent=eth0` is configured as followed:

```
ip addr show eth0
3: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP group default qlen 1000
    inet 172.16.86.250/24 brd 172.16.86.255 scope global eth0
```

Create the macvlan network and run a couple of containers attached to it:

```
# Macvlan  (-o macvlan_mode= Defaults to Bridge mode if not specified)
docker network create -d macvlan \
    --subnet=172.16.86.0/24 \
    --gateway=172.16.86.1  \
    -o parent=eth0 pub_net

# Run a container on the new network specifying the --ip address.
docker  run --net=pub_net --ip=172.16.86.10 -itd alpine /bin/sh

# Start a second container and ping the first
docker  run --net=pub_net -it --rm alpine /bin/sh
ping -c 4 172.16.86.10

```

 Take a look at the containers ip and routing table:
 
```

ip a show eth0
    eth0@if3: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UNKNOWN
    link/ether 46:b2:6b:26:2f:69 brd ff:ff:ff:ff:ff:ff
    inet 172.16.86.2/24 scope global eth0
    
ip route
    default via 172.16.86.1 dev eth0
    172.16.86.0/24 dev eth0  src 172.16.86.2

# NOTE: the containers can NOT ping the underlying host interfaces as
# they are intentionally filtered by Linux for additional isolation.
# In this case the containers cannot ping the -o parent=172.16.86.250
```

You can explicitly specify the `bridge` mode option `-o macvlan_mode=bridge`. It is the default so will be in `bridge` mode either way.

While the `eth0` interface does not need to have an IP address in Macvlan Bridge it is not uncommon to have an IP address on the interface. Addresses can be excluded from getting an address from the default built in IPAM by using the `--aux-address=x.x.x.x` flag. This will blacklist the specified address from being handed out to containers. The same network example above blocking the `-o parent=eth0` address from being handed out to a container.

```
docker network create -d macvlan \
    --subnet=172.16.86.0/24 \
    --gateway=172.16.86.1  \
    --aux-address="exclude_host=172.16.86.250" \
    -o parent=eth0 pub_net
```

Another option for subpool IP address selection in a network provided by the default Docker IPAM driver is to use `--ip-range=`. This specifies the driver to allocate container addresses from this pool rather then the broader range from the `--subnet=` argument from a network create as seen in the following example that will allocate addresses beginning at `192.168.32.128` and increment upwards from there.

```
docker network create -d macvlan  \
    --subnet=192.168.32.0/24  \
    --ip-range=192.168.32.128/25 \
    --gateway=192.168.32.254  \
    -o parent=eth0 macnet32

# Start a container and verify the address is 192.168.32.128
docker run --net=macnet32 -it --rm alpine /bin/sh
```

The network can then be deleted with:

```
docker network rm <network_name or id>
```

- **Note:** In Macvlan you are not able to ping or communicate with the default namespace IP address. For example, if you create a container and try to ping the Docker host's `eth0` it will **not** work. That traffic is explicitly filtered by the kernel modules themselves to offer additional provider isolation and security.

For more on Docker networking commands see [Working with Docker network commands](https://docs.docker.com/engine/userguide/networking/work-with-networks/)

### Macvlan 802.1q Trunk Bridge Mode Example Usage

VLANs (Virtual Local Area Networks) have long been a primary means of virtualizing data center networks and are still in virtually all existing networks today. VLANs work by tagging a Layer-2 isolation domain with a 12-bit identifier ranging from 1-4094 that is inserted into a packet header that enables a logical grouping of a single or multiple subnets of both IPv4 and IPv6. It is very common for network operators to separate traffic using VLANs based on a subnet(s) function or security profile such as `web`, `db` or any other isolation needs.

It is very common to have a compute host requirement of running multiple virtual networks concurrently on a host. Linux networking has long supported VLAN tagging, also known by its standard 802.1q, for maintaining datapath isolation between networks. The Ethernet link connected to a Docker host can be configured to support the 802.1q VLAN IDs, by creating Linux sub-interfaces, each one dedicated to a unique VLAN ID.

![Multi Tenant 802.1q Vlans](images/multi_tenant_8021q_vlans.png)

Trunking 802.1q to a Linux host is notoriously painful for many in operations. It requires configuration file changes in order to be persistent through a reboot. If a bridge is involved, a physical NIC needs to be moved into the bridge and the bridge then gets the IP address. This has lead to many a stranded servers since the risk of cutting off access during that convoluted process is high.

Like all of the Docker network drivers, the overarching goal is to alleviate the operational pains of managing network resources. To that end, when a network receives a sub-interface as the parent that does not exist, the drivers create the VLAN tagged interfaces while creating the network.

In the case of a host reboot, instead of needing to modify often complex network configuration files the driver will recreate all network links when the Docker daemon restarts. The driver tracks if it created the VLAN tagged sub-interface originally with the network create and will **only** recreate the sub-interface after a restart or delete `docker network rm` the link if it created it in the first place with `docker network create`.

If the user doesn't want Docker to modify the `-o parent` sub-interface, the user simply needs to pass an existing link that already exists as the parent interface. Parent interfaces such as `eth0` are not deleted, only sub-interfaces that are not master links.

For the driver to add/delete the vlan sub-interfaces the format needs to be `interface_name.vlan_tag`.

For example: `eth0.50` denotes a parent interface of `eth0` with a slave of `eth0.50` tagged with vlan id `50`. The equivalent `ip link` command would be `ip link add link eth0 name eth0.50 type vlan id 50`.

**Vlan ID 50**

In the first network tagged and isolated by the Docker host, `eth0.50` is the parent interface tagged with vlan id `50` specified with `-o parent=eth0.50`. Other naming formats can be used, but the links need to be added and deleted manually using `ip link` or Linux configuration files. As long as the `-o parent` exists anything can be used if compliant with Linux netlink.

```
# now add networks and hosts as you would normally by attaching to the master (sub)interface that is tagged
docker network  create  -d macvlan \
    --subnet=192.168.50.0/24 \
    --gateway=192.168.50.1 \
    -o parent=eth0.50 macvlan50

# In two separate terminals, start a Docker container and the containers can now ping one another.
docker run --net=macvlan50 -it --name macvlan_test5 --rm alpine /bin/sh
docker run --net=macvlan50 -it --name macvlan_test6 --rm alpine /bin/sh
```

**Vlan ID 60**

In the second network, tagged and isolated by the Docker host, `eth0.60` is the parent interface tagged with vlan id `60` specified with `-o parent=eth0.60`. The `macvlan_mode=` defaults to `macvlan_mode=bridge`. It can also be explicitly set with the same result as shown in the next example.

```
# now add networks and hosts as you would normally by attaching to the master (sub)interface that is tagged. 
docker network  create  -d macvlan \
    --subnet=192.168.60.0/24 \
    --gateway=192.168.60.1 \
    -o parent=eth0.60 -o \
    -o macvlan_mode=bridge macvlan60

# In two separate terminals, start a Docker container and the containers can now ping one another.
docker run --net=macvlan60 -it --name macvlan_test7 --rm alpine /bin/sh
docker run --net=macvlan60 -it --name macvlan_test8 --rm alpine /bin/sh
```
**Example:** Multi-Subnet Macvlan 802.1q Trunking

The same as the example before except there is an additional subnet bound to the network that the user can choose to provision containers on. In MacVlan/Bridge mode, containers can only ping one another if they are on the same subnet/broadcast domain unless there is an external router that routes the traffic (answers ARP etc) between the two subnets.

```
### Create multiple L2 subnets
docker network create -d ipvlan \
    --subnet=192.168.210.0/24 \
    --subnet=192.168.212.0/24 \
    --gateway=192.168.210.254  \
    --gateway=192.168.212.254  \
     -o ipvlan_mode=l2 ipvlan210

# Test 192.168.210.0/24 connectivity between containers
docker run --net=ipvlan210 --ip=192.168.210.10 -itd alpine /bin/sh
docker run --net=ipvlan210 --ip=192.168.210.9 -it --rm alpine ping -c 2 192.168.210.10

# Test 192.168.212.0/24 connectivity between containers
docker run --net=ipvlan210 --ip=192.168.212.10 -itd alpine /bin/sh
docker run --net=ipvlan210 --ip=192.168.212.9 -it --rm alpine ping -c 2 192.168.212.10
```

### Dual Stack IPv4 IPv6 Macvlan Bridge Mode

**Example:** Macvlan Bridge mode, 802.1q trunk, VLAN ID: 218, Multi-Subnet, Dual Stack

```
# Create multiple bridge subnets with a gateway of x.x.x.1:
docker network  create  -d macvlan \
    --subnet=192.168.216.0/24 --subnet=192.168.218.0/24 \
    --gateway=192.168.216.1  --gateway=192.168.218.1 \
    --subnet=2001:db8:abc8::/64 --gateway=2001:db8:abc8::10 \
     -o parent=eth0.218 \
     -o macvlan_mode=bridge macvlan216

# Start a container on the first subnet 192.168.216.0/24
docker run --net=macvlan216 --name=macnet216_test --ip=192.168.216.10 -itd alpine /bin/sh

# Start a container on the second subnet 192.168.218.0/24
docker run --net=macvlan216 --name=macnet216_test --ip=192.168.218.10 -itd alpine /bin/sh

# Ping the first container started on the 192.168.216.0/24 subnet
docker run --net=macvlan216 --ip=192.168.216.11 -it --rm alpine /bin/sh
ping 192.168.216.10

# Ping the first container started on the 192.168.218.0/24 subnet
docker run --net=macvlan216 --ip=192.168.218.11 -it --rm alpine /bin/sh
ping 192.168.218.10
```

View the details of one of the containers:

```
docker run --net=macvlan216 --ip=192.168.216.11 -it --rm alpine /bin/sh

root@526f3060d759:/# ip a show eth0
    eth0@if92: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UNKNOWN group default
    link/ether 8e:9a:99:25:b6:16 brd ff:ff:ff:ff:ff:ff
    inet 192.168.216.11/24 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 2001:db8:abc4::8c9a:99ff:fe25:b616/64 scope link tentative
       valid_lft forever preferred_lft forever
    inet6 2001:db8:abc8::2/64 scope link nodad
       valid_lft forever preferred_lft forever

# Specified v4 gateway of 192.168.216.1     
root@526f3060d759:/# ip route
  default via 192.168.216.1 dev eth0
  192.168.216.0/24 dev eth0  proto kernel  scope link  src 192.168.216.11

# Specified v6 gateway of 2001:db8:abc8::10
root@526f3060d759:/# ip -6 route
  2001:db8:abc4::/64 dev eth0  proto kernel  metric 256
  2001:db8:abc8::/64 dev eth0  proto kernel  metric 256
  default via 2001:db8:abc8::10 dev eth0  metric 1024
```

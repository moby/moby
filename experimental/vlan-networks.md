

# Macvlan and Ipvlan Network Drivers

### Getting Started

The Macvlan and Ipvlan drivers are currently in experimental mode in order to incubate Docker users use cases and vet the implementation to ensure a hardened, production ready driver in a future release. Libnetwork now gives users total control over both IPv4 and IPv6 adressing. The VLAN drivers build on top of that in giving operators complete control of layer 2 VLAN tagging and even Ipvlan L3 routing for users interested in underlay network integration. For overlay deployments that abstract away physical constraints see the [multi-host overlay ](https://docs.docker.com/engine/userguide/networking/get-started-overlay/) driver.

Macvlan and Ipvlan are a new twist on the tried and true network virtualization technique. The Linux implementations are extremely lightweight because rather then using the traditional Linux bridge for isolation, they are simply associated to a Linux Ethernet interface or sub-interface to enforce seperation between networks and connectivty to the physical network.

Macvlan and Ipvlan offer a number of unique features and plenty of room for further innovations with the various modes. Two high level advantages of these approaches are, the positive performance implications of bypassing the Linux bridge and the simplicity of having less moving parts. Removing the bridge that traditionally resides in between the Docker host NIC and container interface leaves a very simple setup consisting of container interfaces, attached directly to the Docker host interface. This result is easy access for external facing services as there is no port mappings in these scenarios.


### Pre-Requisites

- The examples on this page are all single host and setup using Docker experimental builds that can be installed with the following instructions: [Install Docker experimental](https://github.com/docker/docker/tree/master/experimental)

- All of the examples can be performed on a single host running Docker. Any examples using a sub-interface like `eth0.10` can be replaced with `eth0` or any other valid parent interface on the Docker host. Sub-interfaces with a `.` are created on the fly. `-o parent` interfaces can also be left out of the `docker network create` all together and the driver will create a `dummy` interface that will enable local host connectivity to perform the examples.

- Kernel requirements:
 
 - To check your current kernel version, use `uname -r` to display your kernel version
 - Macvlan Linux kernel v3.9–3.19 and 4.0+
 - Ipvlan Linux kernel v4.2+ (support for earlier kernels exists but is buggy)


### MacVlan Bridge Mode Example Usage

Macvlan Bridge mode has a unique MAC address per container used to track MAC to port mappings by the Docker host. This is the largest difference from Ipvlan L2 mode which uses the same MAC address as the parent interface for each container `eth0` interface.

- Macvlan and Ipvlan driver networks are attached to a parent Docker host interface. Examples are a physical interface such as `eth0`, a sub-interface for 802.1q VLAN tagging like `eth0.10` (`.10` representing VLAN `10`) or even bonded host adaptors which bundle two Ethernet interfaces into a single logical interface.

- The specified gateway is external to the host provided by the network infrastructure. 

- Each Macvlan Bridge mode Docker network is isolated from one another and there can be only one network attached to a parent interface at a time. There is a theoretical limit of 4,094 sub-interfaces per host adaptor that a Docker network could be attached to.

- It is not recommended to mix ipvlan and macvlan networks on the same `-o parent=` interface. Older kernel versions will throw uninformative netlink errors such as `device is busy`.

- Any container inside the same subnet can talk any other container in the same network without a  gateway in both `macvlan bridge` mode and `ipvlan L2` modes.

- The same `docker network` commands apply to the vlan drivers. Some are irrelevant such as `-icc` or `--set-macaddress` for the Ipvlan driver.

- In Macvlan and Ipvlan L2 mode, containers on separate networks cannot reach one another without an external process routing between the two networks/subnets. This also applies to multiple subnets within the same `docker network`. See Ipvlan L3 mode for inter-subnet communications without a router.

In the following example, `eth0` on the docker host has an IP on the `172.16.86.0/24` network and a default gateway of `172.16.86.1`. The gateway is an external router with an address of `172.16.86.1`. An IP address is not required on the Docker host interface `eth0` in `bridge` mode, it merely needs to be on the proper upstream network to get forwarded by a network switch or network router.

![Simple Macvlan Bridge Mode Example](../docs/userguide/networking/images/macvlan_bridge_simple.png)


**Note** For Macvlan bridge mode and Ipvlan L2 mode the subnet values need to match the NIC's interface of the Docker host. For example, Use the same subnet and gateway of the Docker host ethernet interface that is specified by the `-o parent=` option.

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

# Take a look at the containers ip and routing table

ip a show eth0
    eth0@if3: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UNKNOWN
    link/ether 46:b2:6b:26:2f:69 brd ff:ff:ff:ff:ff:ff
    inet 172.16.86.1/24 scope global eth0
    
ip route
    default via 172.16.86.2 dev eth0
    172.16.86.0/24 dev eth0  src 172.16.86.1

# NOTE: the containers can NOT ping the underlying host interfaces as
# they are intentionally filtered by Linux for additional isolation.
# In this case the containers cannot ping the -o parent=172.16.86.250
```


You can explicitly specify the `bridge` mode option `-o macvlan_mode=bridge`. It is the default so will be in `bridge` mode either way.

While the `eth0` interface does not need to have an IP address in Macvlan Bridge mode or Ipvlan L2 mode it is not uncommon to have an IP address on the interface. Addresses can be excluded from getting an address from the default built in IPAM by using the `--aux-address=x.x.x.x` flag. This will blacklist the specified address from being handed out to containers. The same network example above blocking the `-o parent=eth0` address from being handed out to a container.

```
docker network create -d macvlan \
    --subnet=172.16.86.0/24 \
    --gateway=172.16.86.1  \
    --aux-address="exclude_host=172.16.86.250" \
    -o parent=eth0 pub_net
```

The network can then be deleted with:

```
docker network rm pub_net
```

- **Note:** In both Macvlan and Ipvlan you are not able to ping or communicate with the default namespace IP address. For example, if you create a container and try to ping the Docker host's `eth0` it will **not** work. That traffic is explicitly filtered by the kernel modules themselves to offer additional provider isolation and security.

For more on Docker networking commands see [Working with Docker network commands](https://docs.docker.com/engine/userguide/networking/work-with-networks/)

### Ipvlan L2 Mode Example Usage

The ipvlan `L2` mode example is virtually identical to the macvlan `bridge` mode example. The driver is specified with `-d driver_name` option. In this case `-d ipvlan`

![Simple Ipvlan L2 Mode Example](../docs/userguide/networking/images/ipvlan_l2_simple.png)

The parent interface in the next example `-o parent=eth0` is configured as followed:

```
ip addr show eth0
3: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP group default qlen 1000
    inet 192.168.1.250/24 brd 192.168.1.255 scope global eth0
```

Use the network from the host's interface as the `--subnet` in the `docker network create`. The container will be attached to the same network as the host interface as set via the `-o parent=` option.

Create the ipvlan network and run a container attaching to it:

```
# Ipvlan  (-o ipvlan_mode= Defaults to L2 mode if not specified)
docker network  create -d ipvlan \
    --subnet=192.168.1.0/24 \ 
    --gateway=192.168.1.1 \
    -o ipvlan_mode=l2 \
    -o parent=eth0 db_net

# Start a container on the db_net network
docker  run --net=db_net -it --rm alpine /bin/sh

# NOTE: the containers can NOT ping the underlying host interfaces as
# they are intentionally filtered by Linux for additional isolation.
```

The default mode for Ipvlan is `l2`. The default mode for Macvlan is `bridge`. If `-o ipvlan_mode=` or `-o macvlan_mode=` are left unspecified, the default modes will be used. Similarly, if the `--gateway` is left empty, the first usable address on the network will be set as the gateway. For example, if the subnet provided in the network create is `--subnet=192.168.1.0/24` then the gateway the container receives is `192.168.1.1`.

The following will create the exact same network as the network `db_net` created prior, with the driver defaults for `--gateway=192.168.1.1` and `-o ipvlan_mode=l2`.

```
# Ipvlan  (-o ipvlan_mode= Defaults to L2 mode if not specified)
docker network  create -d ipvlan \
    --subnet=192.168.1.0/24 \ 
    -o parent=eth0 db_net_ipv

# Start a container with an explicit name in daemon mode
docker  run --net=db_net_ipv --name=ipv1 -itd alpine /bin/sh

# Start a second container and ping using the container name
# to see the docker included name resolution functionality
docker  run --net=db_net_ipv --name=ipv2 -it --rm alpine /bin/sh
ping -c 4 ipv1

# NOTE: the containers can NOT ping the underlying host interfaces as
# they are intentionally filtered by Linux for additional isolation.
```

The drivers also support the `--internal` flag that will completely isolate containers on a network from any communications external to that network. Since network isolation is tightly coupled to the network's parent interface the result of leaving the `-o parent=` option off of a network create is the exact same as the `--internal` option. If the parent interface is not specified or the `--internal` flag is used, a netlink type `dummy` parent interface is created for the user and used as the parent interface effectively isolating the network completely.

The following two `docker network create` examples result in identical networks that you can attach container to:

```
# Empty '-o parent=' creates an isolated network
docker network  create -d ipvlan \
    --subnet=192.168.10.0/24 isolated1

# Explicit '--internal' flag is the same:
docker network  create -d ipvlan \
    --subnet=192.168.11.0/24 --internal isolated2

# Even the '--subnet=' can be left empty and the default 
# IPAM subnet of 172.18.0.0/16 will be assigned
docker network  create -d ipvlan isolated3

docker run --net=isolated1 --name=cid1 -it --rm alpine /bin/sh
docker run --net=isolated2 --name=cid2 -it --rm alpine /bin/sh
docker run --net=isolated3 --name=cid3 -it --rm alpine /bin/sh

# To attach to any use `docker exec` and start a shell
docker exec -it cid1 /bin/sh
docker exec -it cid2 /bin/sh
docker exec -it cid3 /bin/sh
```

### Macvlan 802.1q Trunk Bridge Mode Example Usage

VLANs (Virtual Local Area Networks) have long been a primary means of virtualizing data center networks and are still in virtually all existing networks today. VLANs work by tagging a Layer-2 isolation domain with a 12-bit identifier ranging from 1-4094 that is inserted into a packet header that enables a logical grouping of a single or multiple subnets of both IPv4 and IPv6. It is very common for network operators to separate traffic using VLANs based on a subnet(s) function or security profile such as `web`, `db` or any other isolation needs.

It is very common to have a compute host requirement of running multiple virtual networks concurrently on a host. Linux networking has long supported VLAN tagging, also known by it's standard 802.1q, for maintaining datapath isolation between networks. The Ethernet link connected to a Docker host can be configured to support the 802.1q VLAN IDs, by creating Linux sub-interfaces, each one dedicated to a unique VLAN ID.

![Simple Ipvlan L2 Mode Example](../docs/userguide/networking/images/multi_tenant_8021q_vlans.png)

Trunking 802.1q to a host is notoriously painful. It requires configuration file changes in order to be persistent through a reboot and if a bridge is involved a physical NIC needs to be moved into the bridge and the bridge then gets the IP address or risk being cut off from the network.

Like all of the Docker network drivers, the overarching goal is to alleviate the operational pains of managing network resources. To that end, when a network receives a sub-interface as the parent that does not exist, the drivers create the VLAN tagged interfaces while creating the network.

In the case of a host reboot, instead of needing to modify often complex network configuration files the driver will recreate all network links when the Docker daemon restarts. The driver tracks if it created the VLAN tagged sub-interface originally with the network create and will **only** recreate the sub-interface after a restart or delete `docker network rm` the link if it created it in the first place with `docker network create`.

If the user doesn't want Docker to modify the `-o parent` sub-interface, the user simply needs to pass an existing link that already exists as the parent interface. Parent interfaces such as `eth0` are not deleted, only sub-interfaces that are not master links.

For the driver to add/delete the vlan sub-interfaces the format needs to be `interface_name.vlan_tag`.

For example: `eth0.50` denotes a parent interface of `eth0` with a slave of `eth0.50` tagged with vlan id `50`. The equivalent `ip link` command would be `ip link add link eth0 name eth0.50 type vlan id 50`.

Replace the `macvlan` with `ipvlan` in the `-d` driver argument to create macvlan 802.1q trunks. 

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
docker network  create  -d ipvlan \
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

### Ipvlan 802.1q Trunk L2 Mode Example Usage

Architecturally, Ipvlan L2 mode trunking is the same as Macvlan with regard to gateways and L2 path isolation. There are nuances that can be advantageous for CAM table pressure in ToR switches, one MAC per port and MAC exhaustion on a host's parent NIC to name a few. The 802.1q trunk scenario looks the same. Both modes adhere to tagging standards and have seamless integration with the physical network for underlay integration and hardware vendor plugin integrations.

![Simple Ipvlan L2 Mode Example](../docs/userguide/networking/images/multi_tenant_8021q_vlans.png)

The Linux sub-interface tagged with a vlan can either already exist or will be created when you call a `docker network create`. `docker network rm` will delete the sub-interface. Parent interfaces such as `eth0` are not deleted, only sub-interfaces with a netlink parent index > 0.

For the driver to add/delete the vlan sub-interfaces the format needs to be `interface_name.vlan_tag`. Other sub-interface naming can be used as the specified parent, but the link will not be deleted automatically when `docker network rm` is invoked. 

The option to use either existing parent vlan sub-interfaces or let Docker manage them enables the user to either completely manage the Linux interfaces and networking or let Docker create and delete the Vlan parent sub-interfaces (netlink `ip link`) with no effort from the user.

For example: `eth0.10` to denote a sub-interface of `eth0` tagged with vlan id `10`. The equivalent `ip link` command would be `ip link add link eth0 name eth0.10 type vlan id 10`.

The example creates the vlan tagged networks and then start two containers to test connectivity between containers. Different Vlans cannot ping one another without a router routing between the two networks. The default namespace is not reachable per ipvlan design in order to isolate container namespaces from the underlying host.

**Vlan ID 20**

In the first network tagged and isolated by the Docker host, `eth0.20` is the parent interface tagged with vlan id `20` specified with `-o parent=eth0.20`. Other naming formats can be used, but the links need to be added and deleted manually using `ip link` or Linux configuration files. As long as the `-o parent` exists anything can be used if compliant with Linux netlink.

```
# now add networks and hosts as you would normally by attaching to the master (sub)interface that is tagged
docker network  create  -d ipvlan \
    --subnet=192.168.20.0/24 \
    --gateway=192.168.20.1 \
    -o parent=eth0.20 ipvlan20

# in two separate terminals, start a Docker container and the containers can now ping one another.
docker run --net=ipvlan20 -it --name ivlan_test1 --rm alpine /bin/sh
docker run --net=ipvlan20 -it --name ivlan_test2 --rm alpine /bin/sh
```

**Vlan ID 30**

In the second network, tagged and isolated by the Docker host, `eth0.30` is the parent interface tagged with vlan id `30` specified with `-o parent=eth0.30`. The `ipvlan_mode=` defaults to l2 mode `ipvlan_mode=l2`. It can also be explicitly set with the same result as shown in the next example.

```
# now add networks and hosts as you would normally by attaching to the master (sub)interface that is tagged.
docker network  create  -d ipvlan \
    --subnet=192.168.30.0/24 \
    --gateway=192.168.30.1 \
    -o parent=eth0.30 \
    -o ipvlan_mode=l2 ipvlan30

# in two separate terminals, start a Docker container and the containers can now ping one another.
docker run --net=ipvlan30 -it --name ivlan_test3 --rm alpine /bin/sh
docker run --net=ipvlan30 -it --name ivlan_test4 --rm alpine /bin/sh
```

The gateway is set inside of the container as the default gateway. That gateway would typically be an external router on the network.

```
$ ip route
  default via 192.168.30.1 dev eth0
  192.168.30.0/24 dev eth0  src 192.168.30.2
```

Example: Multi-Subnet Ipvlan L2 Mode starting two containers on the same subnet and pinging one another. In order for the `192.168.114.0/24` to reach `192.168.116.0/24` it requires an external router in L2 mode. L3 mode can route between subnets that share a common `-o parent=`. This same multi-subnet example is also valid for Macvlan `bridge` mode.

Secondary addresses on network routers are common as an address space becomes exhausted to add another secondary to a L3 vlan interface or commonly referred to as a "switched virtual interface" (SVI).

```
docker network  create  -d ipvlan \
    --subnet=192.168.114.0/24 --subnet=192.168.116.0/24 \
    --gateway=192.168.114.254  --gateway=192.168.116.254 \
     -o parent=eth0.114 \
     -o ipvlan_mode=l2 ipvlan114
     
docker run --net=ipvlan114 --ip=192.168.114.10 -it --rm alpine /bin/sh
docker run --net=ipvlan114 --ip=192.168.114.11 -it --rm alpine /bin/sh
```

A key takeaway is, operators have the ability to map their physical network into their virtual network for integrating containers into their environment with no operational overhauls required. NetOps simply drops an 802.1q trunk into the Docker host. That virtual link would be the `-o parent=` passed in the network creation. For untagged (non-VLAN) links, it is as simple as `-o parent=eth0` or for 802.1q trunks with VLAN IDs each network gets mapped to the corresponding VLAN/Subnet from the network.

An example being, NetOps provides VLAN ID and the associated subnets for VLANs being passed on the Ethernet link to the Docker host server. Those values are simply plugged into the `docker network create` commands when provisioning the Docker networks. These are persistent configurations that are applied every time the Docker engine starts which alleviates having to manage often complex configuration files. The network interfaces can also be managed manually by being pre-created and docker networking will never modify them, simply use them as parent interfaces. Example mappings from NetOps to Docker network commands are as follows:

- VLAN: 10, Subnet: 172.16.80.0/24, Gateway: 172.16.80.1

    - `--subnet=172.16.80.0/24 --gateway=172.16.80.1 -o parent=eth0.10` 

- VLAN: 20, IP subnet: 172.16.50.0/22, Gateway: 172.16.50.1

    - `--subnet=172.16.50.0/22 --gateway=172.16.50.1 -o parent=eth0.20 ` 

- VLAN: 30, Subnet: 10.1.100.0/16, Gateway: 10.1.100.1

    - `--subnet=10.1.100.0/16 --gateway=10.1.100.1 -o parent=eth0.30` 

### IPVlan L3 Mode Example

IPVlan will require routes to be distributed to each endpoint. The driver only builds the Ipvlan L3 mode port and attaches the container to the interface. Route distribution throughout a cluster is beyond the initial implementation of this single host scoped driver. In L3 mode the Docker host is very similar to a router starting new networks in the container. They are on networks that the upstream network will not know about without route distribution. For those curious how Ipvlan L3 will fit into container networking see the following examples.

- Ipvlan L3 mode drops all broadcast and multicast traffic.

- L3 mode needs to be on a separate subnet as the default namespace since it requires a netlink route in the default namespace pointing to the Ipvlan parent interface.

- The parent interface used in this example is `eth0` and it is on the subnet `192.168.1.0/24`. Notice the `docker network` is **not** on the same subnet as `eth0`.

- Unlike macvlan bridge mode and ipvlan l2 modes, different subnets/networks can ping one another as long as they share the same parent interface `-o parent=`.

```
ip a show eth0
3: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP group default qlen 1000
    link/ether 00:50:56:39:45:2e brd ff:ff:ff:ff:ff:ff
    inet 192.168.1.250/24 brd 192.168.1.255 scope global eth0
```

-A traditional gateway doesn't mean much to an L3 mode Ipvlan interface since there is no broadcast traffic allowed. Because of that, the container default gateway simply point the the containers `eth0` device. See below for CLI output of `ip route` or `ip -6 route` from inside an L3 container for details.

The mode ` -o ipvlan_mode=l3` must be explicitly specified since the default ipvlan mode is `l2`.

```
# Create the Ipvlan L3 network
docker network  create  -d ipvlan \
    --subnet=192.168.214.0/24 \
    --subnet=10.1.214.0/24 \
     -o ipvlan_mode=l3 ipnet210

# Test 192.168.214.0/24 connectivity
docker run --net=ipnet210 --ip=192.168.214.10 -itd alpine /bin/sh
docker run --net=ipnet210 --ip=10.1.214.10 -itd alpine /bin/sh

# Test L3 connectivity from 10.1.214.0/24 to 192.168.212.0/24
docker run --net=ipnet210 --ip=192.168.214.9 -it --rm alpine ping -c 2 10.1.214.10

# Test L3 connectivity from 192.168.212.0/24 to 10.1.214.0/24
docker run --net=ipnet210 --ip=10.1.214.9 -it --rm alpine ping -c 2 192.168.214.10

```

Notice there is no `--gateway=` option in the network create. The field is ignored if one is specified `l3` mode. Take a look at the container routing table from inside of the container:

```
# Inside an L3 mode container
$ ip route
  default dev eth0
  192.168.120.0/24 dev eth0  src 192.168.120.2
```

In order to ping the containers from a remote Docker host or the container be able to ping a remote host, the remote host or the physical network in between need to have a route pointing to the host IP address of the container's Docker host eth interface. More on this as we evolve the Ipvlan `L3` story.

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

docker run --net=macvlan216 --name=macnet216_test --ip=192.168.216.10 -itd alpine /bin/sh
docker run --net=macvlan216 --name=macnet216_test --ip=192.168.218.10 -itd alpine /bin/sh
docker run --net=macvlan216 --ip=192.168.216.11 -it --rm alpine /bin/sh
docker run --net=macvlan216 --ip=192.168.218.11 -it --rm alpine /bin/sh
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

### Dual Stack IPv4 IPv6 Ipvlan L2 Mode

- Not only does Libnetwork give you complete control over IPv4 addressing, but it also gives you total control over IPv6 addressing as well as feature parity between the two address families.

- The next example will start with IPv6 only. Start two containers on the same VLAN `139` and ping one another. Since the IPv4 subnet is not specified, the default IPAM will provision a default IPv4 subnet. That subnet is isolated unless the upstream network is explicitly routing it on VLAN `139`.

```
# Create a v6 network
docker network create -d ipvlan \
    --subnet=2001:db8:abc2::/64 --gateway=2001:db8:abc2::22 \
    -o parent=eth0.139 v6ipvlan139
    
# Start a container on the network
docker run --net=v6ipvlan139 -it --rm alpine /bin/sh

```

View the container eth0 interface and v6 routing table:

```
 eth0@if55: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UNKNOWN group default
    link/ether 00:50:56:2b:29:40 brd ff:ff:ff:ff:ff:ff
    inet 172.18.0.2/16 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 2001:db8:abc4::250:56ff:fe2b:2940/64 scope link
       valid_lft forever preferred_lft forever
    inet6 2001:db8:abc2::1/64 scope link nodad
       valid_lft forever preferred_lft forever
       
root@5c1dc74b1daa:/# ip -6 route
2001:db8:abc4::/64 dev eth0  proto kernel  metric 256
2001:db8:abc2::/64 dev eth0  proto kernel  metric 256
default via 2001:db8:abc2::22 dev eth0  metric 1024
```

Start a second container and ping the first container's v6 address. 

```
$ docker run --net=v6ipvlan139 -it --rm alpine /bin/sh

root@b817e42fcc54:/# ip a show eth0
75: eth0@if55: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UNKNOWN group default
    link/ether 00:50:56:2b:29:40 brd ff:ff:ff:ff:ff:ff
    inet 172.18.0.3/16 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 2001:db8:abc4::250:56ff:fe2b:2940/64 scope link tentative dadfailed
       valid_lft forever preferred_lft forever
    inet6 2001:db8:abc2::2/64 scope link nodad
       valid_lft forever preferred_lft forever

root@b817e42fcc54:/# ping6 2001:db8:abc2::1
PING 2001:db8:abc2::1 (2001:db8:abc2::1): 56 data bytes
64 bytes from 2001:db8:abc2::1%eth0: icmp_seq=0 ttl=64 time=0.044 ms
64 bytes from 2001:db8:abc2::1%eth0: icmp_seq=1 ttl=64 time=0.058 ms

2 packets transmitted, 2 packets received, 0% packet loss
round-trip min/avg/max/stddev = 0.044/0.051/0.058/0.000 ms
```

The next example with setup a dual stack IPv4/IPv6 network with an example VLAN ID of `140`.

Next create a network with two IPv4 subnets and one IPv6 subnets, all of which have explicit gateways:

```
docker network  create  -d ipvlan \
    --subnet=192.168.140.0/24 --subnet=192.168.142.0/24 \
    --gateway=192.168.140.1  --gateway=192.168.142.1 \
    --subnet=2001:db8:abc9::/64 --gateway=2001:db8:abc9::22 \
     -o parent=eth0.140 \
     -o ipvlan_mode=l2 ipvlan140
```

Start a container and view eth0 and both v4 & v6 routing tables:

```
docker run --net=v6ipvlan139 --ip6=2001:db8:abc2::51 -it --rm alpine /bin/sh

root@3cce0d3575f3:/# ip a show eth0
78: eth0@if77: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UNKNOWN group default
    link/ether 00:50:56:2b:29:40 brd ff:ff:ff:ff:ff:ff
    inet 192.168.140.2/24 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 2001:db8:abc4::250:56ff:fe2b:2940/64 scope link
       valid_lft forever preferred_lft forever
    inet6 2001:db8:abc9::1/64 scope link nodad
       valid_lft forever preferred_lft forever

root@3cce0d3575f3:/# ip route
default via 192.168.140.1 dev eth0
192.168.140.0/24 dev eth0  proto kernel  scope link  src 192.168.140.2

root@3cce0d3575f3:/# ip -6 route
2001:db8:abc4::/64 dev eth0  proto kernel  metric 256
2001:db8:abc9::/64 dev eth0  proto kernel  metric 256
default via 2001:db8:abc9::22 dev eth0  metric 1024
```

Start a second container with a specific `--ip4` address and ping the first host using ipv4 packets:

```
docker run --net=ipvlan140 --ip=192.168.140.10 -it --rm alpine /bin/sh
```

**Note**: Different subnets on the same parent interface in both Ipvlan `L2` mode and Macvlan `bridge` mode cannot ping one another. That requires a router to proxy-arp the requests with a secondary subnet. However, Ipvlan `L3` will route the unicast traffic between disparate subnets as long as they share the same `-o parent` parent link.



### Dual Stack IPv4 IPv6 Ipvlan L3 Mode 


**Example:** IpVlan L3 Mode Dual Stack IPv4/IPv6, Multi-Subnet w/ 802.1q Vlan Tag:118

As in all of the examples, a tagged VLAN interface does not have to be used. The sub-interfaces can be swapped with `eth0`, `eth1` or any other valid interface on the host other then the `lo` loopback.

The primary difference you will see is that L3 mode does not create a default route with a next-hop but rather sets a default route pointing to `dev eth` only since ARP/Broadcasts/Multicast are all filtered by Linux as per the design.

```
# Create an IPv6+IPv4 Dual Stack Ipvlan L3 network 
# Gateways for both v4 and v6 are set to a dev e.g. 'default dev eth0'
docker network  create  -d ipvlan \
    --subnet=192.168.110.0/24 \
    --subnet=192.168.112.0/24 \
    --subnet=2001:db8:abc6::/64 \
     -o parent=eth0.118 \
     -o ipvlan_mode=l3 ipnet110


# Start a few of containers on the network (ipnet110) 
# in seperate terminals and check connectivity
docker run --net=ipnet110 -it --rm alpine /bin/sh
# Start a second container specifying the v6 address
docker run --net=ipnet110 --ip6=2001:db8:abc6::10 -it --rm alpine /bin/sh
# Start a third specifying the IPv4 address
docker run --net=ipnet110 --ip=192.168.112.50 -it --rm alpine /bin/sh
# Start a 4th specifying both the IPv4 and IPv6 addresses
docker run --net=ipnet110 --ip6=2001:db8:abc6::50 --ip=192.168.112.50 -it --rm alpine /bin/sh
```

Interface and routing table outputs are as follows:

```
root@3a368b2a982e:/# ip a show eth0
63: eth0@if59: <BROADCAST,MULTICAST,NOARP,UP,LOWER_UP> mtu 1500 qdisc noqueue state UNKNOWN group default
    link/ether 00:50:56:2b:29:40 brd ff:ff:ff:ff:ff:ff
    inet 192.168.112.2/24 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 2001:db8:abc4::250:56ff:fe2b:2940/64 scope link
       valid_lft forever preferred_lft forever
    inet6 2001:db8:abc6::10/64 scope link nodad
       valid_lft forever preferred_lft forever
     
# Note the default route is simply the eth device because ARPs are filtered.
root@3a368b2a982e:/# ip route
  default dev eth0  scope link
  192.168.112.0/24 dev eth0  proto kernel  scope link  src 192.168.112.2

root@3a368b2a982e:/# ip -6 route
2001:db8:abc4::/64 dev eth0  proto kernel  metric 256
2001:db8:abc6::/64 dev eth0  proto kernel  metric 256
default dev eth0  metric 1024
```

*Note:* There may be a bug when specifying `--ip6=` addresses when you delete a container with a specified v6 address and then start a new container with the same v6 address it throws the following like the address isn't properly being released to the v6 pool. It will fail to unmount the container and be left dead.

```
docker: Error response from daemon: Address already in use.
```

### Manually Creating 802.1q Links

**Vlan ID 40**

If a user does not want the driver to create the vlan sub-interface it simply needs to exist prior to the `docker network create`. If you have sub-interface naming that is not `interface.vlan_id` it is honored in the `-o parent=` option again as long as the interface exists and us up.

Links if manually created can be named anything you want. As long as the exist when the network is created that is all that matters. Manually created links do not get deleted regardless of the name when the network is deleted with `docker network rm`.

```
# create a new sub-interface tied to dot1q vlan 40
ip link add link eth0 name eth0.40 type vlan id 40

# enable the new sub-interface
ip link set eth0.40 up

# now add networks and hosts as you would normally by attaching to the master (sub)interface that is tagged
docker network  create  -d ipvlan \
   --subnet=192.168.40.0/24 \
   --gateway=192.168.40.1 \
   -o parent=eth0.40 ipvlan40

# in two separate terminals, start a Docker container and the containers can now ping one another.
docker run --net=ipvlan40 -it --name ivlan_test5 --rm alpine /bin/sh
docker run --net=ipvlan40 -it --name ivlan_test6 --rm alpine /bin/sh
```

**Example:** Vlan sub-interface manually created with any name:

```
# create a new sub interface tied to dot1q vlan 40
ip link add link eth0 name foo type vlan id 40

# enable the new sub-interface
ip link set foo up

# now add networks and hosts as you would normally by attaching to the master (sub)interface that is tagged
docker network  create  -d ipvlan \
    --subnet=192.168.40.0/24 --gateway=192.168.40.1 \
    -o parent=foo ipvlan40

# in two separate terminals, start a Docker container and the containers can now ping one another.
docker run --net=ipvlan40 -it --name ivlan_test5 --rm alpine /bin/sh
docker run --net=ipvlan40 -it --name ivlan_test6 --rm alpine /bin/sh
```

Manually created links can be cleaned up with:

```
ip link del foo
```

As with all of the Libnetwork drivers, they can be mixed and matched, even as far as running 3rd party ecosystem drivers in parallel for maximum flexibility to the Docker user.

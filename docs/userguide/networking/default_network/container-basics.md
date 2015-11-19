<!--[metadata]>
+++
draft=true
title = "Configure container DNS"
description = "Learn how to configure DNS in Docker"
keywords = ["docker, bridge, docker0, network"]
[menu.main]
parent = "smn_networking_def"
+++
<![end-metadata]-->

<!--[metadata]>
DRAFT to prevent building. Keeping for one cycle before deleting.
<![end-metadata]-->

# How the default network

The information in this section explains configuring container DNS within tthe Docker default bridge. This is a `bridge` network named `bridge` created
automatically when you install Docker.

**Note**: The [Docker networks feature](../dockernetworks.md) allows you to create user-defined networks in addition to the default bridge network.

While Docker is under active development and continues to tweak and improve its network configuration logic, the shell commands in this section are rough equivalents to the steps that Docker takes when configuring networking for each new container.

## Review some basics

To communicate using the Internet Protocol (IP), a machine needs access to at least one network interface at which packets can be sent and received, and a routing table that defines the range of IP addresses reachable through that interface.  Network interfaces do not have to be physical devices.  In fact, the `lo` loopback interface available on every Linux machine (and inside each Docker container) is entirely virtual -- the Linux kernel simply copies loopback packets directly from the sender's memory into the receiver's memory.

Docker uses special virtual interfaces to let containers communicate with the host machine -- pairs of virtual interfaces called "peers" that are linked inside of the host machine's kernel so that packets can travel between them.  They are simple to create, as we will see in a moment.

The steps with which Docker configures a container are:
- Create a pair of peer virtual interfaces.
- Give one of them a unique name like `veth65f9`, keep it inside of the main Docker host, and bind it to `docker0` or whatever bridge Docker is supposed to be using.

- Toss the other interface over the wall into the new container (which will already have been provided with an `lo` interface) and rename it to the much prettier name `eth0` since, inside of the container's separate and unique network interface namespace, there are no physical interfaces with which this name could collide.

- Set the interface's MAC address according to the `--mac-address` parameter or generate a random one.

- Give the container's `eth0` a new IP address from within the bridge's range of network addresses. The default route is set to the IP address passed to the Docker daemon using the `--default-gateway` option if specified, otherwise to the IP address that the Docker host owns on the bridge. The MAC address is generated from the IP address unless otherwise specified. This prevents ARP cache invalidation problems, when a new container comes up with an IP used in the past by another container with another MAC.

With these steps complete, the container now possesses an `eth0` (virtual) network card and will find itself able to communicate with other containers and the rest of the Internet.

You can opt out of the above process for a particular container by giving the `--net=` option to `docker run`, which takes four possible values.
- `--net=bridge` -- The default action, that connects the container to the Docker bridge as described above.

- `--net=host` -- Tells Docker to skip placing the container inside of a separate network stack.  In essence, this choice tells Docker to **not containerize the container's networking**!  While container processes will still be confined to their own filesystem and process list and resource limits, a quick `ip addr` command will show you that, network-wise, they live "outside" in the main Docker host and have full access to its network interfaces.  Note that this does **not** let the container reconfigure the host network stack -- that would require `--privileged=true` -- but it does let container processes open low-numbered ports like any other root process. It also allows the container to access local network services like D-bus.  This can lead to processes in the container being able to do unexpected things like [restart your computer](https://github.com/docker/docker/issues/6401). You should use this option with caution.

- `--net=container:NAME_or_ID` -- Tells Docker to put this container's processes inside of the network stack that has already been created inside of another container.  The new container's processes will be confined to their own filesystem and process list and resource limits, but will share the same IP address and port numbers as the first container, and processes on the two containers will be able to connect to each other over the loopback interface.

- `--net=none` -- Tells Docker to put the container inside of its own network stack but not to take any steps to configure its network, leaving you free to build any of the custom configurations explored in the last few sections of this document.

## Manually network

To get an idea of the steps that are necessary if you use `--net=none` as described in that last bullet point, here are the commands that you would run to reach roughly the same configuration as if you had let Docker do all of the configuration:

```
# At one shell, start a container and
# leave its shell idle and running

$ docker run -i -t --rm --net=none base /bin/bash
root@63f36fc01b5f:/#

# At another shell, learn the container process ID
# and create its namespace entry in /var/run/netns/
# for the "ip netns" command we will be using below

$ docker inspect -f '{{.State.Pid}}' 63f36fc01b5f
2778
$ pid=2778
$ sudo mkdir -p /var/run/netns
$ sudo ln -s /proc/$pid/ns/net /var/run/netns/$pid

# Check the bridge's IP address and netmask

$ ip addr show docker0
21: docker0: ...
inet 172.17.42.1/16 scope global docker0
...

# Create a pair of "peer" interfaces A and B,
# bind the A end to the bridge, and bring it up

$ sudo ip link add A type veth peer name B
$ sudo brctl addif docker0 A
$ sudo ip link set A up

# Place B inside the container's network namespace,
# rename to eth0, and activate it with a free IP

$ sudo ip link set B netns $pid
$ sudo ip netns exec $pid ip link set dev B name eth0
$ sudo ip netns exec $pid ip link set eth0 address 12:34:56:78:9a:bc
$ sudo ip netns exec $pid ip link set eth0 up
$ sudo ip netns exec $pid ip addr add 172.17.42.99/16 dev eth0
$ sudo ip netns exec $pid ip route add default via 172.17.42.1
```

At this point your container should be able to perform networking operations as usual.

When you finally exit the shell and Docker cleans up the container, the network namespace is destroyed along with our virtual `eth0` -- whose destruction in turn destroys interface `A` out in the Docker host and automatically un-registers it from the `docker0` bridge.  So everything gets cleaned up without our having to run any extra commands!  Well, almost everything:

```
# Clean up dangling symlinks in /var/run/netns

find -L /var/run/netns -type l -delete
```

Also note that while the script above used modern `ip` command instead of old deprecated wrappers like `ipconfig` and `route`, these older commands would also have worked inside of our container.  The `ip addr` command can be typed as `ip a` if you are in a hurry.

Finally, note the importance of the `ip netns exec` command, which let us reach inside and configure a network namespace as root.  The same commands would not have worked if run inside of the container, because part of safe containerization is that Docker strips container processes of the right to configure their own networks.  Using `ip netns exec` is what let us finish up the configuration without having to take the dangerous step of running the container itself with `--privileged=true`.

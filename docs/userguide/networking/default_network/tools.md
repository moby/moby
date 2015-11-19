<!--[metadata]>
+++
draft=true
title = "Tools and Examples"
keywords = ["docker, bridge, docker0, network"]
[menu.main]
parent = "smn_networking_def"
+++
<![end-metadata]-->

<!--[metadata]>
Dave Tucker instructed remove this.  We may want to add it back in later under another form. Labeled DRAFT for now. Won't be built.
<![end-metadata]-->

# Tools and examples
Before diving into the following sections on custom network topologies, you might be interested in glancing at a few external tools or examples of the same kinds of configuration.  Here are two:
- Jérôme Petazzoni has created a `pipework` shell script to help you

  connect together containers in arbitrarily complex scenarios:

  [https://github.com/jpetazzo/pipework](https://github.com/jpetazzo/pipework)

- Brandon Rhodes has created a whole network topology of Docker

  containers for the next edition of Foundations of Python Network

  Programming that includes routing, NAT'd firewalls, and servers that

  offer HTTP, SMTP, POP, IMAP, Telnet, SSH, and FTP:

  [https://github.com/brandon-rhodes/fopnp/tree/m/playground](https://github.com/brandon-rhodes/fopnp/tree/m/playground)

Both tools use networking commands very much like the ones you saw in the previous section, and will see in the following sections.

# Building a point-to-point connection
<a name="point-to-point"></a>

By default, Docker attaches all containers to the virtual subnet implemented by `docker0`.  You can create containers that are each connected to some different virtual subnet by creating your own bridge as shown in [Building your own bridge](#bridge-building), starting each container with `docker run --net=none`, and then attaching the containers to your bridge with the shell commands shown in [How Docker networks a container](#container-networking).

But sometimes you want two particular containers to be able to communicate directly without the added complexity of both being bound to a host-wide Ethernet bridge.

The solution is simple: when you create your pair of peer interfaces, simply throw _both_ of them into containers, and configure them as classic point-to-point links.  The two containers will then be able to communicate directly (provided you manage to tell each container the other's IP address, of course).  You might adjust the instructions of the previous section to go something like this:

```
# Start up two containers in two terminal windows

$ docker run -i -t --rm --net=none base /bin/bash
root@1f1f4c1f931a:/#

$ docker run -i -t --rm --net=none base /bin/bash
root@12e343489d2f:/#

# Learn the container process IDs
# and create their namespace entries

$ docker inspect -f '{{.State.Pid}}' 1f1f4c1f931a
2989
$ docker inspect -f '{{.State.Pid}}' 12e343489d2f
3004
$ sudo mkdir -p /var/run/netns
$ sudo ln -s /proc/2989/ns/net /var/run/netns/2989
$ sudo ln -s /proc/3004/ns/net /var/run/netns/3004

# Create the "peer" interfaces and hand them out

$ sudo ip link add A type veth peer name B

$ sudo ip link set A netns 2989
$ sudo ip netns exec 2989 ip addr add 10.1.1.1/32 dev A
$ sudo ip netns exec 2989 ip link set A up
$ sudo ip netns exec 2989 ip route add 10.1.1.2/32 dev A

$ sudo ip link set B netns 3004
$ sudo ip netns exec 3004 ip addr add 10.1.1.2/32 dev B
$ sudo ip netns exec 3004 ip link set B up
$ sudo ip netns exec 3004 ip route add 10.1.1.1/32 dev B
```

The two containers should now be able to ping each other and make connections successfully.  Point-to-point links like this do not depend on a subnet nor a netmask, but on the bare assertion made by `ip route` that some other single IP address is connected to a particular network interface.

Note that point-to-point links can be safely combined with other kinds of network connectivity -- there is no need to start the containers with `--net=none` if you want point-to-point links to be an addition to the container's normal networking instead of a replacement.

A final permutation of this pattern is to create the point-to-point link between the Docker host and one container, which would allow the host to communicate with that one container on some single IP address and thus communicate "out-of-band" of the bridge that connects the other, more usual containers.  But unless you have very specific networking needs that drive you to such a solution, it is probably far preferable to use `--icc=false` to lock down inter-container communication, as we explored earlier.

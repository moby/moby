page_title: Network Configuration
page_description: Docker networking
page_keywords: network, networking, bridge, docker, documentation

# Network Configuration

## TL;DR

When Docker starts, it creates a virtual interface named `docker0` on
the host machine.  It randomly chooses an address and subnet from the
private range defined by [RFC 1918](http://tools.ietf.org/html/rfc1918)
that are not in use on the host machine, and assigns it to `docker0`.
Docker made the choice `172.17.42.1/16` when I started it a few minutes
ago, for example — a 16-bit netmask providing 65,534 addresses for the
host machine and its containers.

> **Note:** 
> This document discusses advanced networking configuration
> and options for Docker. In most cases you won't need this information.
> If you're looking to get started with a simpler explanation of Docker
> networking and an introduction to the concept of container linking see
> the [Docker User Guide](/userguide/dockerlinks/).

But `docker0` is no ordinary interface.  It is a virtual *Ethernet
bridge* that automatically forwards packets between any other network
interfaces that are attached to it.  This lets containers communicate
both with the host machine and with each other.  Every time Docker
creates a container, it creates a pair of “peer” interfaces that are
like opposite ends of a pipe — a packet send on one will be received on
the other.  It gives one of the peers to the container to become its
`eth0` interface and keeps the other peer, with a unique name like
`vethAQI2QT`, out in the namespace of the host machine.  By binding
every `veth*` interface to the `docker0` bridge, Docker creates a
virtual subnet shared between the host machine and every Docker
container.

The remaining sections of this document explain all of the ways that you
can use Docker options and — in advanced cases — raw Linux networking
commands to tweak, supplement, or entirely replace Docker's default
networking configuration.

## Quick Guide to the Options

Here is a quick list of the networking-related Docker command-line
options, in case it helps you find the section below that you are
looking for.

Some networking command-line options can only be supplied to the Docker
server when it starts up, and cannot be changed once it is running:

 *  `-b BRIDGE` or `--bridge=BRIDGE` — see
    [Building your own bridge](#bridge-building)

 *  `--bip=CIDR` — see
    [Customizing docker0](#docker0)

 *  `-H SOCKET...` or `--host=SOCKET...` —
    This might sound like it would affect container networking,
    but it actually faces in the other direction:
    it tells the Docker server over what channels
    it should be willing to receive commands
    like “run container” and “stop container.”

 *  `--icc=true|false` — see
    [Communication between containers](#between-containers)

 *  `--ip=IP_ADDRESS` — see
    [Binding container ports](#binding-ports)

 *  `--ip-forward=true|false` — see
    [Communication between containers](#between-containers)

 *  `--iptables=true|false` — see
    [Communication between containers](#between-containers)

 *  `--mtu=BYTES` — see
    [Customizing docker0](#docker0)

There are two networking options that can be supplied either at startup
or when `docker run` is invoked.  When provided at startup, set the
default value that `docker run` will later use if the options are not
specified:

 *  `--dns=IP_ADDRESS...` — see
    [Configuring DNS](#dns)

 *  `--dns-search=DOMAIN...` — see
    [Configuring DNS](#dns)

Finally, several networking options can only be provided when calling
`docker run` because they specify something specific to one container:

 *  `-h HOSTNAME` or `--hostname=HOSTNAME` — see
    [Configuring DNS](#dns) and
    [How Docker networks a container](#container-networking)

 *  `--link=CONTAINER_NAME:ALIAS` — see
    [Configuring DNS](#dns) and
    [Communication between containers](#between-containers)

 *  `--net=bridge|none|container:NAME_or_ID|host` — see
    [How Docker networks a container](#container-networking)

 *  `-p SPEC` or `--publish=SPEC` — see
    [Binding container ports](#binding-ports)

 *  `-P` or `--publish-all=true|false` — see
    [Binding container ports](#binding-ports)

The following sections tackle all of the above topics in an order that
moves roughly from simplest to most complex.

## <a name="dns"></a>Configuring DNS

How can Docker supply each container with a hostname and DNS
configuration, without having to build a custom image with the hostname
written inside?  Its trick is to overlay three crucial `/etc` files
inside the container with virtual files where it can write fresh
information.  You can see this by running `mount` inside a container:

    $$ mount
    ...
    /dev/disk/by-uuid/1fec...ebdf on /etc/hostname type ext4 ...
    /dev/disk/by-uuid/1fec...ebdf on /etc/hosts type ext4 ...
    tmpfs on /etc/resolv.conf type tmpfs ...
    ...

This arrangement allows Docker to do clever things like keep
`resolv.conf` up to date across all containers when the host machine
receives new configuration over DHCP later.  The exact details of how
Docker maintains these files inside the container can change from one
Docker version to the next, so you should leave the files themselves
alone and use the following Docker options instead.

Four different options affect container domain name services.

 *  `-h HOSTNAME` or `--hostname=HOSTNAME` — sets the hostname by which
    the container knows itself.  This is written into `/etc/hostname`,
    into `/etc/hosts` as the name of the container’s host-facing IP
    address, and is the name that `/bin/bash` inside the container will
    display inside its prompt.  But the hostname is not easy to see from
    outside the container.  It will not appear in `docker ps` nor in the
    `/etc/hosts` file of any other container.

 *  `--link=CONTAINER_NAME:ALIAS` — using this option as you `run` a
    container gives the new container’s `/etc/hosts` an extra entry
    named `ALIAS` that points to the IP address of the container named
    `CONTAINER_NAME`.  This lets processes inside the new container
    connect to the hostname `ALIAS` without having to know its IP.  The
    `--link=` option is discussed in more detail below, in the section
    [Communication between containers](#between-containers).

 *  `--dns=IP_ADDRESS...` — sets the IP addresses added as `server`
    lines to the container's `/etc/resolv.conf` file.  Processes in the
    container, when confronted with a hostname not in `/etc/hosts`, will
    connect to these IP addresses on port 53 looking for name resolution
    services.

 *  `--dns-search=DOMAIN...` — sets the domain names that are searched
    when a bare unqualified hostname is used inside of the container, by
    writing `search` lines into the container’s `/etc/resolv.conf`.
    When a container process attempts to access `host` and the search
    domain `exmaple.com` is set, for instance, the DNS logic will not
    only look up `host` but also `host.example.com`.

Note that Docker, in the absence of either of the last two options
above, will make `/etc/resolv.conf` inside of each container look like
the `/etc/resolv.conf` of the host machine where the `docker` daemon is
running.  The options then modify this default configuration.

## <a name="between-containers"></a>Communication between containers

Whether two containers can communicate is governed, at the operating
system level, by three factors.

1.  Does the network topology even connect the containers’ network
    interfaces?  By default Docker will attach all containers to a
    single `docker0` bridge, providing a path for packets to travel
    between them.  See the later sections of this document for other
    possible topologies.

2.  Is the host machine willing to forward IP packets?  This is governed
    by the `ip_forward` system parameter.  Packets can only pass between
    containers if this parameter is `1`.  Usually you will simply leave
    the Docker server at its default setting `--ip-forward=true` and
    Docker will go set `ip_forward` to `1` for you when the server
    starts up.  To check the setting or turn it on manually:

        # Usually not necessary: turning on forwarding,
        # on the host where your Docker server is running

        $ cat /proc/sys/net/ipv4/ip_forward
        0
        $ sudo echo 1 > /proc/sys/net/ipv4/ip_forward
        $ cat /proc/sys/net/ipv4/ip_forward
        1

3.  Do your `iptables` allow this particular connection to be made?
    Docker will never make changes to your system `iptables` rules if
    you set `--iptables=false` when the daemon starts.  Otherwise the
    Docker server will add a default rule to the `FORWARD` chain with a
    blanket `ACCEPT` policy if you retain the default `--icc=true`, or
    else will set the policy to `DROP` if `--icc=false`.

Nearly everyone using Docker will want `ip_forward` to be on, to at
least make communication *possible* between containers.  But it is a
strategic question whether to leave `--icc=true` or change it to
`--icc=false` (on Ubuntu, by editing the `DOCKER_OPTS` variable in
`/etc/default/docker` and restarting the Docker server) so that
`iptables` will protect other containers — and the main host — from
having arbitrary ports probed or accessed by a container that gets
compromised.

If you choose the most secure setting of `--icc=false`, then how can
containers communicate in those cases where you *want* them to provide
each other services?

The answer is the `--link=CONTAINER_NAME:ALIAS` option, which was
mentioned in the previous section because of its effect upon name
services.  If the Docker daemon is running with both `--icc=false` and
`--iptables=true` then, when it sees `docker run` invoked with the
`--link=` option, the Docker server will insert a pair of `iptables`
`ACCEPT` rules so that the new container can connect to the ports
exposed by the other container — the ports that it mentioned in the
`EXPOSE` lines of its `Dockerfile`.  Docker has more documentation on
this subject — see the [linking Docker containers](/userguide/dockerlinks)
page for further details.

> **Note**:
> The value `CONTAINER_NAME` in `--link=` must either be an
> auto-assigned Docker name like `stupefied_pare` or else the name you
> assigned with `--name=` when you ran `docker run`.  It cannot be a
> hostname, which Docker will not recognize in the context of the
> `--link=` option.

You can run the `iptables` command on your Docker host to see whether
the `FORWARD` chain has a default policy of `ACCEPT` or `DROP`:

    # When --icc=false, you should see a DROP rule:

    $ sudo iptables -L -n
    ...
    Chain FORWARD (policy ACCEPT)
    target     prot opt source               destination
    DROP       all  --  0.0.0.0/0            0.0.0.0/0
    ...

    # When a --link= has been created under --icc=false,
    # you should see port-specific ACCEPT rules overriding
    # the subsequent DROP policy for all other packets:

    $ sudo iptables -L -n
    ...
    Chain FORWARD (policy ACCEPT)
    target     prot opt source               destination
    ACCEPT     tcp  --  172.17.0.2           172.17.0.3           tcp spt:80
    ACCEPT     tcp  --  172.17.0.3           172.17.0.2           tcp dpt:80
    DROP       all  --  0.0.0.0/0            0.0.0.0/0

> **Note**:
> Docker is careful that its host-wide `iptables` rules fully expose
> containers to each other’s raw IP addresses, so connections from one
> container to another should always appear to be originating from the
> first container’s own IP address.

## <a name="binding-ports"></a>Binding container ports to the host

By default Docker containers can make connections to the outside world,
but the outside world cannot connect to containers.  Each outgoing
connection will appear to originate from one of the host machine’s own
IP addresses thanks to an `iptables` masquerading rule on the host
machine that the Docker server creates when it starts:

    # You can see that the Docker server creates a
    # masquerade rule that let containers connect
    # to IP addresses in the outside world:

    $ sudo iptables -t nat -L -n
    ...
    Chain POSTROUTING (policy ACCEPT)
    target     prot opt source               destination
    MASQUERADE  all  --  172.17.0.0/16       !172.17.0.0/16
    ...

But if you want containers to accept incoming connections, you will need
to provide special options when invoking `docker run`.  These options
are covered in more detail in the [Docker User Guide](/userguide/dockerlinks)
page.  There are two approaches.

First, you can supply `-P` or `--publish-all=true|false` to `docker run`
which is a blanket operation that identifies every port with an `EXPOSE`
line in the image’s `Dockerfile` and maps it to a host port somewhere in
the range 49000–49900.  This tends to be a bit inconvenient, since you
then have to run other `docker` sub-commands to learn which external
port a given service was mapped to.

More convenient is the `-p SPEC` or `--publish=SPEC` option which lets
you be explicit about exactly which external port on the Docker server —
which can be any port at all, not just those in the 49000–49900 block —
you want mapped to which port in the container.

Either way, you should be able to peek at what Docker has accomplished
in your network stack by examining your NAT tables.

    # What your NAT rules might look like when Docker
    # is finished setting up a -P forward:

    $ iptables -t nat -L -n
    ...
    Chain DOCKER (2 references)
    target     prot opt source               destination
    DNAT       tcp  --  0.0.0.0/0            0.0.0.0/0            tcp dpt:49153 to:172.17.0.2:80

    # What your NAT rules might look like when Docker
    # is finished setting up a -p 80:80 forward:

    Chain DOCKER (2 references)
    target     prot opt source               destination
    DNAT       tcp  --  0.0.0.0/0            0.0.0.0/0            tcp dpt:80 to:172.17.0.2:80

You can see that Docker has exposed these container ports on `0.0.0.0`,
the wildcard IP address that will match any possible incoming port on
the host machine.  If you want to be more restrictive and only allow
container services to be contacted through a specific external interface
on the host machine, you have two choices.  When you invoke `docker run`
you can use either `-p IP:host_port:container_port` or `-p IP::port` to
specify the external interface for one particular binding.

Or if you always want Docker port forwards to bind to one specific IP
address, you can edit your system-wide Docker server settings (on
Ubuntu, by editing `DOCKER_OPTS` in `/etc/default/docker`) and add the
option `--ip=IP_ADDRESS`.  Remember to restart your Docker server after
editing this setting.

Again, this topic is covered without all of these low-level networking
details in the [Docker User Guide](/userguide/dockerlinks/) document if you
would like to use that as your port redirection reference instead.

## <a name="docker0"></a>Customizing docker0

By default, the Docker server creates and configures the host system’s
`docker0` interface as an *Ethernet bridge* inside the Linux kernel that
can pass packets back and forth between other physical or virtual
network interfaces so that they behave as a single Ethernet network.

Docker configures `docker0` with an IP address and netmask so the host
machine can both receive and send packets to containers connected to the
bridge, and gives it an MTU — the *maximum transmission unit* or largest
packet length that the interface will allow — of either 1,500 bytes or
else a more specific value copied from the Docker host’s interface that
supports its default route.  Both are configurable at server startup:

 *  `--bip=CIDR` — supply a specific IP address and netmask for the
    `docker0` bridge, using standard CIDR notation like
    `192.168.1.5/24`.

 *  `--mtu=BYTES` — override the maximum packet length on `docker0`.

On Ubuntu you would add these to the `DOCKER_OPTS` setting in
`/etc/default/docker` on your Docker host and restarting the Docker
service.

Once you have one or more containers up and running, you can confirm
that Docker has properly connected them to the `docker0` bridge by
running the `brctl` command on the host machine and looking at the
`interfaces` column of the output.  Here is a host with two different
containers connected:

    # Display bridge info

    $ sudo brctl show
    bridge name     bridge id               STP enabled     interfaces
    docker0         8000.3a1d7362b4ee       no              veth65f9
                                                            vethdda6

If the `brctl` command is not installed on your Docker host, then on
Ubuntu you should be able to run `sudo apt-get install bridge-utils` to
install it.

Finally, the `docker0` Ethernet bridge settings are used every time you
create a new container.  Docker selects a free IP address from the range
available on the bridge each time you `docker run` a new container, and
configures the container’s `eth0` interface with that IP address and the
bridge’s netmask.  The Docker host’s own IP address on the bridge is
used as the default gateway by which each container reaches the rest of
the Internet.

    # The network, as seen from a container

    $ sudo docker run -i -t --rm base /bin/bash

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

Remember that the Docker host will not be willing to forward container
packets out on to the Internet unless its `ip_forward` system setting is
`1` — see the section above on [Communication between
containers](#between-containers) for details.

## <a name="bridge-building"></a>Building your own bridge

If you want to take Docker out of the business of creating its own
Ethernet bridge entirely, you can set up your own bridge before starting
Docker and use `-b BRIDGE` or `--bridge=BRIDGE` to tell Docker to use
your bridge instead.  If you already have Docker up and running with its
old `bridge0` still configured, you will probably want to begin by
stopping the service and removing the interface:

    # Stopping Docker and removing docker0

    $ sudo service docker stop
    $ sudo ip link set dev docker0 down
    $ sudo brctl delbr docker0

Then, before starting the Docker service, create your own bridge and
give it whatever configuration you want.  Here we will create a simple
enough bridge that we really could just have used the options in the
previous section to customize `docker0`, but it will be enough to
illustrate the technique.

    # Create our own bridge

    $ sudo brctl addbr bridge0
    $ sudo ip addr add 192.168.5.1/24 dev bridge0
    $ sudo ip link set dev bridge0 up

    # Confirming that our bridge is up and running

    $ ip addr show bridge0
    4: bridge0: <BROADCAST,MULTICAST> mtu 1500 qdisc noop state UP group default
        link/ether 66:38:d0:0d:76:18 brd ff:ff:ff:ff:ff:ff
        inet 192.168.5.1/24 scope global bridge0
           valid_lft forever preferred_lft forever

    # Tell Docker about it and restart (on Ubuntu)

    $ echo 'DOCKER_OPTS="-b=bridge0"' >> /etc/default/docker
    $ sudo service docker start

The result should be that the Docker server starts successfully and is
now prepared to bind containers to the new bridge.  After pausing to
verify the bridge’s configuration, try creating a container — you will
see that its IP address is in your new IP address range, which Docker
will have auto-detected.

Just as we learned in the previous section, you can use the `brctl show`
command to see Docker add and remove interfaces from the bridge as you
start and stop containers, and can run `ip addr` and `ip route` inside a
container to see that it has been given an address in the bridge’s IP
address range and has been told to use the Docker host’s IP address on
the bridge as its default gateway to the rest of the Internet.

## <a name="container-networking"></a>How Docker networks a container

While Docker is under active development and continues to tweak and
improve its network configuration logic, the shell commands in this
section are rough equivalents to the steps that Docker takes when
configuring networking for each new container.

Let’s review a few basics.

To communicate using the Internet Protocol (IP), a machine needs access
to at least one network interface at which packets can be sent and
received, and a routing table that defines the range of IP addresses
reachable through that interface.  Network interfaces do not have to be
physical devices.  In fact, the `lo` loopback interface available on
every Linux machine (and inside each Docker container) is entirely
virtual — the Linux kernel simply copies loopback packets directly from
the sender’s memory into the receiver’s memory.

Docker uses special virtual interfaces to let containers communicate
with the host machine — pairs of virtual interfaces called “peers” that
are linked inside of the host machine’s kernel so that packets can
travel between them.  They are simple to create, as we will see in a
moment.

The steps with which Docker configures a container are:

1.  Create a pair of peer virtual interfaces.

2.  Give one of them a unique name like `veth65f9`, keep it inside of
    the main Docker host, and bind it to `docker0` or whatever bridge
    Docker is supposed to be using.

3.  Toss the other interface over the wall into the new container (which
    will already have been provided with an `lo` interface) and rename
    it to the much prettier name `eth0` since, inside of the container’s
    separate and unique network interface namespace, there are no
    physical interfaces with which this name could collide.

4.  Give the container’s `eth0` a new IP address from within the
    bridge’s range of network addresses, and set its default route to
    the IP address that the Docker host owns on the bridge.

With these steps complete, the container now possesses an `eth0`
(virtual) network card and will find itself able to communicate with
other containers and the rest of the Internet.

You can opt out of the above process for a particular container by
giving the `--net=` option to `docker run`, which takes four possible
values.

 *  `--net=bridge` — The default action, that connects the container to
    the Docker bridge as described above.

 *  `--net=host` — Tells Docker to skip placing the container inside of
    a separate network stack.  In essence, this choice tells Docker to
    **not containerize the container’s networking**!  While container
    processes will still be confined to their own filesystem and process
    list and resource limits, a quick `ip addr` command will show you
    that, network-wise, they live “outside” in the main Docker host and
    have full access to its network interfaces.  Note that this does
    **not** let the container reconfigure the host network stack — that
    would require `--privileged=true` — but it does let container
    processes open low-numbered ports like any other root process.

 *  `--net=container:NAME_or_ID` — Tells Docker to put this container’s
    processes inside of the network stack that has already been created
    inside of another container.  The new container’s processes will be
    confined to their own filesystem and process list and resource
    limits, but will share the same IP address and port numbers as the
    first container, and processes on the two containers will be able to
    connect to each other over the loopback interface.

 *  `--net=none` — Tells Docker to put the container inside of its own
    network stack but not to take any steps to configure its network,
    leaving you free to build any of the custom configurations explored
    in the last few sections of this document.

To get an idea of the steps that are necessary if you use `--net=none`
as described in that last bullet point, here are the commands that you
would run to reach roughly the same configuration as if you had let
Docker do all of the configuration:

    # At one shell, start a container and
    # leave its shell idle and running

    $ sudo docker run -i -t --rm --net=none base /bin/bash
    root@63f36fc01b5f:/#

    # At another shell, learn the container process ID
    # and create its namespace entry in /var/run/netns/
    # for the "ip netns" command we will be using below

    $ sudo docker inspect -f '{{.State.Pid}}' 63f36fc01b5f
    2778
    $ pid=2778
    $ sudo mkdir -p /var/run/netns
    $ sudo ln -s /proc/$pid/ns/net /var/run/netns/$pid

    # Check the bridge’s IP address and netmask

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
    $ sudo ip netns exec $pid ip link set eth0 up
    $ sudo ip netns exec $pid ip addr add 172.17.42.99/16 dev eth0
    $ sudo ip netns exec $pid ip route add default via 172.17.42.1

At this point your container should be able to perform networking
operations as usual.

When you finally exit the shell and Docker cleans up the container, the
network namespace is destroyed along with our virtual `eth0` — whose
destruction in turn destroys interface `A` out in the Docker host and
automatically un-registers it from the `docker0` bridge.  So everything
gets cleaned up without our having to run any extra commands!  Well,
almost everything:

    # Clean up dangling symlinks in /var/run/netns

    find -L /var/run/netns -type l -delete

Also note that while the script above used modern `ip` command instead
of old deprecated wrappers like `ipconfig` and `route`, these older
commands would also have worked inside of our container.  The `ip addr`
command can be typed as `ip a` if you are in a hurry.

Finally, note the importance of the `ip netns exec` command, which let
us reach inside and configure a network namespace as root.  The same
commands would not have worked if run inside of the container, because
part of safe containerization is that Docker strips container processes
of the right to configure their own networks.  Using `ip netns exec` is
what let us finish up the configuration without having to take the
dangerous step of running the container itself with `--privileged=true`.

## Tools and Examples

Before diving into the following sections on custom network topologies,
you might be interested in glancing at a few external tools or examples
of the same kinds of configuration.  Here are two:

 *  Jérôme Petazzoni has created a `pipework` shell script to help you
    connect together containers in arbitrarily complex scenarios:
    <https://github.com/jpetazzo/pipework>

 *  Brandon Rhodes has created a whole network topology of Docker
    containers for the next edition of Foundations of Python Network
    Programming that includes routing, NAT’d firewalls, and servers that
    offer HTTP, SMTP, POP, IMAP, Telnet, SSH, and FTP:
    <https://github.com/brandon-rhodes/fopnp/tree/m/playground>

Both tools use networking commands very much like the ones you saw in
the previous section, and will see in the following sections.

## <a name="point-to-point"></a>Building a point-to-point connection

By default, Docker attaches all containers to the virtual subnet
implemented by `docker0`.  You can create containers that are each
connected to some different virtual subnet by creating your own bridge
as shown in [Building your own bridge](#bridge-building), starting each
container with `docker run --net=none`, and then attaching the
containers to your bridge with the shell commands shown in [How Docker
networks a container](#container-networking).

But sometimes you want two particular containers to be able to
communicate directly without the added complexity of both being bound to
a host-wide Ethernet bridge.

The solution is simple: when you create your pair of peer interfaces,
simply throw *both* of them into containers, and configure them as
classic point-to-point links.  The two containers will then be able to
communicate directly (provided you manage to tell each container the
other’s IP address, of course).  You might adjust the instructions of
the previous section to go something like this:

    # Start up two containers in two terminal windows

    $ sudo docker run -i -t --rm --net=none base /bin/bash
    root@1f1f4c1f931a:/#

    $ sudo docker run -i -t --rm --net=none base /bin/bash
    root@12e343489d2f:/#

    # Learn the container process IDs
    # and create their namespace entries

    $ sudo docker inspect -f '{{.State.Pid}}' 1f1f4c1f931a
    2989
    $ sudo docker inspect -f '{{.State.Pid}}' 12e343489d2f
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

The two containers should now be able to ping each other and make
connections sucessfully.  Point-to-point links like this do not depend
on a subnet nor a netmask, but on the bare assertion made by `ip route`
that some other single IP address is connected to a particular network
interface.

Note that point-to-point links can be safely combined with other kinds
of network connectivity — there is no need to start the containers with
`--net=none` if you want point-to-point links to be an addition to the
container’s normal networking instead of a replacement.

A final permutation of this pattern is to create the point-to-point link
between the Docker host and one container, which would allow the host to
communicate with that one container on some single IP address and thus
communicate “out-of-band” of the bridge that connects the other, more
usual containers.  But unless you have very specific networking needs
that drive you to such a solution, it is probably far preferable to use
`--icc=false` to lock down inter-container communication, as we explored
earlier.

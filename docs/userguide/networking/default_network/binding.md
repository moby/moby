<!--[metadata]>
+++
title = "Bind container ports to the host"
description = "expose, port, docker, bind publish"
keywords = ["Examples, Usage, network, docker, documentation, user guide, multihost, cluster"]
[menu.main]
parent = "smn_networking_def"
+++
<![end-metadata]-->

# Bind container ports to the host

The information in this section explains binding container ports within the Docker default bridge. This is a `bridge` network named `bridge` created automatically when you install Docker.

> **Note**: The [Docker networks feature](../dockernetworks.md) allows you to
create user-defined networks in addition to the default bridge network.

By default Docker containers can make connections to the outside world, but the
outside world cannot connect to containers. Each outgoing connection will
appear to originate from one of the host machine's own IP addresses thanks to an
`iptables` masquerading rule on the host machine that the Docker server creates
when it starts:

```
$ sudo iptables -t nat -L -n
...
Chain POSTROUTING (policy ACCEPT)
target     prot opt source               destination
MASQUERADE  all  --  172.17.0.0/16       0.0.0.0/0
...
```
The Docker server creates a masquerade rule that let containers connect to IP
addresses in the outside world.

If you want containers to accept incoming connections, you will need to provide
special options when invoking `docker run`. There are two approaches.

First, you can supply `-P` or `--publish-all=true|false` to `docker run` which
is a blanket operation that identifies every port with an `EXPOSE` line in the
image's `Dockerfile` or `--expose <port>` commandline flag and maps it to a host
port somewhere within an _ephemeral port range_. The `docker port` command then
needs to be used to inspect created mapping. The _ephemeral port range_ is
configured by `/proc/sys/net/ipv4/ip_local_port_range` kernel parameter,
typically ranging from 32768 to 61000.

Mapping can be specified explicitly using `-p SPEC` or `--publish=SPEC` option.
It allows you to particularize which port on docker server - which can be any
port at all, not just one within the _ephemeral port range_ -- you want mapped
to which port in the container.

Either way, you should be able to peek at what Docker has accomplished in your
network stack by examining your NAT tables.

```
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
```

You can see that Docker has exposed these container ports on `0.0.0.0`, the
wildcard IP address that will match any possible incoming port on the host
machine. If you want to be more restrictive and only allow container services to
be contacted through a specific external interface on the host machine, you have
two choices. When you invoke `docker run` you can use either `-p
IP:host_port:container_port` or `-p IP::port` to specify the external interface
for one particular binding.

Or if you always want Docker port forwards to bind to one specific IP address,
you can edit your system-wide Docker server settings and add the option
`--ip=IP_ADDRESS`. Remember to restart your Docker server after editing this
setting.

> **Note**: With hairpin NAT enabled (`--userland-proxy=false`), containers port
exposure is achieved purely through iptables rules, and no attempt to bind the
exposed port is ever made. This means that nothing prevents shadowing a
previously listening service outside of Docker through exposing the same port
for a container. In such conflicting situation, Docker created iptables rules
will take precedence and route to the container.

The `--userland-proxy` parameter, true by default, provides a userland
implementation for inter-container and outside-to-container communication. When
disabled, Docker uses both an additional `MASQUERADE` iptable rule and the
`net.ipv4.route_localnet` kernel parameter which allow the host machine to
connect to a local container exposed port through the commonly used loopback
address: this alternative is preferred for performance reasons.

## Related information

- [Understand Docker container networks](../dockernetworks.md)
- [Work with network commands](../work-with-networks.md)
- [Legacy container links](dockerlinks.md)

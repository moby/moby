<!--[metadata]>
+++
title = "Build your own bridge"
description = "Learn how to build your own bridge interface"
keywords = ["docker, bridge, docker0, network"]
[menu.main]
parent = "smn_networking_def"
+++
<![end-metadata]-->

# Build your own bridge

This section explains building your own bridge to replaced the Docker default
bridge. This is a `bridge` network named `bridge` created automatically when you
install Docker.

> **Note**: The [Docker networks feature](../dockernetworks.md) allows you to
create user-defined networks in addition to the default bridge network.

You can set up your own bridge before starting Docker and use `-b BRIDGE` or
`--bridge=BRIDGE` to tell Docker to use your bridge instead.  If you already
have Docker up and running with its default `docker0` still configured, you will
probably want to begin by stopping the service and removing the interface:

```
# Stopping Docker and removing docker0

$ sudo service docker stop
$ sudo ip link set dev docker0 down
$ sudo brctl delbr docker0
$ sudo iptables -t nat -F POSTROUTING
```

Then, before starting the Docker service, create your own bridge and give it
whatever configuration you want.  Here we will create a simple enough bridge
that we really could just have used the options in the previous section to
customize `docker0`, but it will be enough to illustrate the technique.

```
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

# Confirming new outgoing NAT masquerade is set up

$ sudo iptables -t nat -L -n
...
Chain POSTROUTING (policy ACCEPT)
target     prot opt source               destination
MASQUERADE  all  --  192.168.5.0/24      0.0.0.0/0
```

The result should be that the Docker server starts successfully and is now
prepared to bind containers to the new bridge.  After pausing to verify the
bridge's configuration, try creating a container -- you will see that its IP
address is in your new IP address range, which Docker will have auto-detected.

You can use the `brctl show` command to see Docker add and remove interfaces
from the bridge as you start and stop containers, and can run `ip addr` and `ip
route` inside a container to see that it has been given an address in the
bridge's IP address range and has been told to use the Docker host's IP address
on the bridge as its default gateway to the rest of the Internet.

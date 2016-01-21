% DOCKER(1) Docker User Manuals
% Docker Community
% OCT 2015
# NAME
docker-network-connect - connect a container to a network

# SYNOPSIS
**docker network connect**
[**--help**]
NETWORK CONTAINER

# DESCRIPTION

Connects a container to a network. You can connect a container by name
or by ID. Once connected, the container can communicate with other containers in
the same network.

```bash
$ docker network connect multi-host-network container1
```

You can also use the `docker run --net=<network-name>` option to start a container and immediately connect it to a network.

```bash
$ docker run -itd --net=multi-host-network --ip 172.20.88.22 --ip6 2001:db8::8822 busybox
```

You can pause, restart, and stop containers that are connected to a network.
Paused containers remain connected and can be revealed by a `network inspect`.
When the container is stopped, it does not appear on the network until you restart
it. If specified, the container's IP address(es) will be reapplied (if still available)
when a stopped container rejoins the network. One way to guarantee that the container
will be assigned the same IP addresses when it rejoins the network after a stop
or a disconnect, is to specify the `--ip-range` when creating the network, and choose
the static IP address(es) from outside the range. This will ensure that the IP address
will not be given to other dynamic containers while this container is not on the network.

```bash
$ docker network create --subnet 172.20.0.0/16 --ip-range 172.20.240.0/20 multi-host-network
```

```bash
$ docker network connect --ip 172.20.128.2 multi-host-network container2
```

To verify the container is connected, use the `docker network inspect` command. Use `docker network disconnect` to remove a container from the network.

Once connected in network, containers can communicate using only another
container's IP address or name. For `overlay` networks or custom plugins that
support multi-host connectivity, containers connected to the same multi-host
network but launched from different Engines can also communicate in this way.

You can connect a container to one or more networks. The networks need not be the same type. For example, you can connect a single container bridge and overlay networks.


# OPTIONS
**NETWORK**
  Specify network name

**CONTAINER**
  Specify container name

**--help**
  Print usage statement

# HISTORY
OCT 2015, created by Mary Anthony <mary@docker.com>

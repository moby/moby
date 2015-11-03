% DOCKER(1) Docker User Manuals
% Docker Community
% OCT 2015
# NAME
docker-network-connect - connect a container to a network

# SYNOPSIS
**docker network connect NAME CONTAINER**

[**--help**]

# DESCRIPTION

Connects a running container to a network. You can connect a container by name
or by ID. Once connected, the container can communicate with other containers in
the same network.

```bash
$ docker network connect multi-host-network container1
```

You can also use the `docker run --net=<network-name>` option to start a container and immediately connect it to a network.

```bash
$ docker run -itd --net=multi-host-network busybox
```

You can pause, restart, and stop containers that are connected to a network.
Paused containers remain connected and a revealed by a `network inspect`. When
the container is stopped, it does not appear on the network until you restart
it. The container's IP address is not guaranteed to remain the same when a
stopped container rejoins the network.

To verify the container is connected, use the `docker network inspect` command. Use `docker network disconnect` to remove a container from the network.

Once connected in network, containers can communicate using only another
container's IP address or name. For `overlay` networks or custom plugins that
support multi-host connectivity, containers connected to the same multi-host
network but launched from different Engines can also communicate in this way.

You can connect a container to one or more networks. The networks need not be the same type. For example, you can connect a single container bridge and overlay networks.


# OPTIONS
**NAME**
  Specify network driver name

**CONTAINER**
  Specify container name

**--help**
  Print usage statement

# HISTORY
OCT 2015, created by Mary Anthony <mary@docker.com>

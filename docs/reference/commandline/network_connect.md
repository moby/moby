<!--[metadata]>
+++
title = "network connect"
description = "The network connect command description and usage"
keywords = ["network, connect, user-defined"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# network connect

    Usage:  docker network connect [OPTIONS] NETWORK CONTAINER

    Connects a container to a network

      --help             Print usage
      --ip               IP Address
      --ip6              IPv6 Address
      --link=[]          Add a link to another container

Connects a container to a network. You can connect a container by name
or by ID. Once connected, the container can communicate with other containers in
the same network.

```bash
$ docker network connect multi-host-network container1
```

You can also use the `docker run --net=<network-name>` option to start a container and immediately connect it to a network.

```bash
$ docker run -itd --net=multi-host-network busybox
```

You can specify the IP address you want to be assigned to the container's interface.

```bash
$ docker network connect --ip 10.10.36.122 multi-host-network container2
```

You can use `--link` option to link another container with a prefered alias

```bash
$ docker network connect --link container1:c1 multi-host-network container2
```

You can pause, restart, and stop containers that are connected to a network.
Paused containers remain connected and a revealed by a `network inspect`. When
the container is stopped, it does not appear on the network until you restart
it. The container's IP address is not guaranteed to remain the same when a
stopped container rejoins the network, unless you specified one when you run
`docker network connect` command.

To verify the container is connected, use the `docker network inspect` command. Use `docker network disconnect` to remove a container from the network.

Once connected in network, containers can communicate using only another
container's IP address or name. For `overlay` networks or custom plugins that
support multi-host connectivity, containers connected to the same multi-host
network but launched from different Engines can also communicate in this way.

You can connect a container to one or more networks. The networks need not be the same type. For example, you can connect a single container bridge and overlay networks.

## Related information

* [network inspect](network_inspect.md)
* [network create](network_create.md)
* [network disconnect](network_disconnect.md)
* [network ls](network_ls.md)
* [network rm](network_rm.md)
* [Understand Docker container networks](../../userguide/networking/dockernetworks.md)
* [Work with networks](../../userguide/networking/work-with-networks.md)

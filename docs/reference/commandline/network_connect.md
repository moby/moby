---
title: "network connect"
description: "The network connect command description and usage"
keywords: ["network, connect, user-defined"]
---

# network connect

```markdown
Usage:  docker network connect [OPTIONS] NETWORK CONTAINER

Connect a container to a network

Options:
      --alias value           Add network-scoped alias for the container (default [])
      --help                  Print usage
      --ip string             IP Address
      --ip6 string            IPv6 Address
      --link value            Add link to another container (default [])
      --link-local-ip value   Add a link-local address for the container (default [])
```

Connects a container to a network. You can connect a container by name
or by ID. Once connected, the container can communicate with other containers in
the same network.

```bash
$ docker network connect multi-host-network container1
```

You can also use the `docker run --network=<network-name>` option to start a container and immediately connect it to a network.

```bash
$ docker run -itd --network=multi-host-network busybox
```

You can specify the IP address you want to be assigned to the container's interface.

```bash
$ docker network connect --ip 10.10.36.122 multi-host-network container2
```

You can use `--link` option to link another container with a preferred alias

```bash
$ docker network connect --link container1:c1 multi-host-network container2
```

`--alias` option can be used to resolve the container by another name in the network
being connected to.

```bash
$ docker network connect --alias db --alias mysql multi-host-network container2
```

You can pause, restart, and stop containers that are connected to a network.
Paused containers remain connected and can be revealed by a `network inspect`.
When the container is stopped, it does not appear on the network until you restart
it.

If specified, the container's IP address(es) is reapplied when a stopped
container is restarted. If the IP address is no longer available, the container
fails to start. One way to guarantee that the IP address is available is
to specify an `--ip-range` when creating the network, and choose the static IP
address(es) from outside that range. This ensures that the IP address is not
given to another container while this container is not on the network.

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

## Related information

* [network inspect](network_inspect.md)
* [network create](network_create.md)
* [network disconnect](network_disconnect.md)
* [network ls](network_ls.md)
* [network rm](network_rm.md)
* [Understand Docker container networks](../../userguide/networking/index.md)
* [Work with networks](../../userguide/networking/work-with-networks.md)

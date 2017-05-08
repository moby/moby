<!--[metadata]>
+++
title = "Swarm mode overlay network security model"
description = "Docker swarm mode overlay network security model"
keywords = ["network, docker, documentation, user guide, multihost, swarm mode", "overlay"]
[menu.main]
parent = "smn_networking"
weight=-2
+++
<![end-metadata]-->

# Docker swarm mode overlay network security model

Overlay networking for Docker Engine swarm mode comes secure out of the box. The
swarm nodes exchange overlay network information using a gossip protocol. By
default the nodes encrypt and authenticate information they exchange via gossip
using the [AES algorithm](https://en.wikipedia.org/wiki/Galois/Counter_Mode) in
GCM mode. Manager nodes in the swarm rotate the key used to encrypt gossip data
every 12 hours.

You can also encrypt data exchanged between containers on different nodes on the
overlay network. To enable encryption, when you create an overlay network pass
the `--opt encrypted` flag:

```bash
$ docker network create --opt encrypted --driver overlay my-multi-host-network

dt0zvqn0saezzinc8a5g4worx
```

When you enable overlay encryption, Docker creates IPSEC tunnels between all the
nodes where tasks are scheduled for services attached to the overlay network.
These tunnels also use the AES algorithm in GCM mode and manager nodes
automatically rotate the keys every 12 hours.

## Swarm mode overlay networks and unmanaged containers

Because the overlay networks for swarm mode use encryption keys from the manager
nodes to encrypt the gossip communications, only containers running as tasks in
the swarm have access to the keys. Consequently, containers started outside of
swarm mode using `docker run` (unmanaged containers) cannot attach to the
overlay network.

For example:

```bash
$ docker run --network my-multi-host-network nginx

docker: Error response from daemon: swarm-scoped network
(my-multi-host-network) is not compatible with `docker create` or `docker
run`. This network can only be used by a docker service.
```

To work around this situation, migrate the unmanaged containers to managed
services. For instance:

```bash
$ docker service create --network my-multi-host-network my-image
```

Because [swarm mode](../../swarm/index.md) is an optional feature, the Docker
Engine preserves backward compatibility. You can continue to rely on a
third-party key-value store to support overlay networking if you wish.
However, switching to swarm-mode is strongly encouraged. In addition to the
security benefits described in this article, swarm mode enables you to leverage
the substantially greater scalability provided by the new services API.

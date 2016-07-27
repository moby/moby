<!--[metadata]>
+++
title = "Run Docker Engine in swarm mode"
description = "Run Docker Engine in swarm mode"
keywords = ["guide, swarm mode, node"]
[menu.main]
identifier="initialize-swarm-guide"
parent="engine_swarm"
weight=12
+++
<![end-metadata]-->

# Run Docker Engine in swarm mode

When you first install and start working with Docker Engine, swarm mode is
disabled by default. When you enable swarm mode, you work with the concept of
services managed through the `docker service` command.

There are two ways to run the Engine in swarm mode:

* Create a new swarm, covered in this article.
* [Join an existing swarm](join-nodes.md).

When you run the Engine in swarm mode on your local machine, you can create and
test services based upon images you've created or other available images. In
your production environment, swarm mode provides a fault-tolerant platform with
cluster management features to keep your services running and available.

These instructions assume you have installed the Docker Engine 1.12 or later on
a machine to serve as a manager node in your swawrm.

If you haven't already, read through the [swarm mode key concepts](key-concepts.md)
and try the [swarm mode tutorial](swarm-tutorial/index.md).

## Create a swarm

When you run the command to create a swarm, the Docker Engine starts running in swarm mode.

Run [`docker swarm init`](/engine/reference/commandline/swarm_init.md)]
to create a single-node swarm on the current node. The Engine sets up the swarm
as follows:

* switches the current node into swarm mode.
* creates a swarm named `default`.
* designates the current node as a leader manager node for the swarm.
* names the node with the machine hostname.
* configures the manager to listen on an active network interface on port 2377.
* sets the current node to `Active` availability, meanining it can receive tasks
from the scheduler.
* starts an internal distributed data store for Engines participating in the
swarm to maintain a consistent view of the swarm and all services running on it.
* by default, generates a self-signed root CA for the swarm.
* by default, generates tokens for worker and manager nodes to join the
swarm.
* creates an overlay network named `ingress` for publishing service ports
external to the swarm.

The output for `docker swarm init` provides the connection command to use when
you join new worker or manager nodes to the swarm:

```bash
$ docker swarm init
Swarm initialized: current node (dxn1zf6l61qsb1josjja83ngz) is now a manager.

To add a worker to this swarm, run the following command:
    docker swarm join \
    --token SWMTKN-1-49nj1cmql0jkz5s954yi3oex3nedyz0fb0xx14ie39trti4wxv-8vxv8rssmk743ojnwacrr2e7c \
    192.168.99.100:2377

To add a manager to this swarm, run the following command:
    docker swarm join \
    --token SWMTKN-1-61ztec5kyafptydic6jfc1i33t37flcl4nuipzcusor96k7kby-5vy9t8u35tuqm7vh67lrz9xp6 \
    192.168.99.100:2377
```

### View the join command or update a swarm join token

The manager node requires a secret token for a new node to join the swarm. The
token for worker nodes is different from the token for manager nodes.

To retrieve the join command including the join token for worker nodes, run:

```bash
$ docker swarm join-token worker

To add a worker to this swarm, run the following command:
    docker swarm join \
    --token SWMTKN-1-49nj1cmql0jkz5s954yi3oex3nedyz0fb0xx14ie39trti4wxv-8vxv8rssmk743ojnwacrr2e7c \
    192.168.99.100:2377

This node joined a swarm as a worker.
```

To view the join command and token for manager nodes, run:

```bash
$ docker swarm join-token manager

To add a worker to this swarm, run the following command:
    docker swarm join \
    --token SWMTKN-1-49nj1cmql0jkz5s954yi3oex3nedyz0fb0xx14ie39trti4wxv-8vxv8rssmk743ojnwacrr2e7c \
    192.168.99.100:2377
```

Pass the `--quiet` flag to print only the token:

```bash
$ docker swarm join-token --quiet worker

SWMTKN-1-49nj1cmql0jkz5s954yi3oex3nedyz0fb0xx14ie39trti4wxv-8vxv8rssmk743ojnwacrr2e7c
```

Pass the `--rotate` for `swarm join-token` to the token for a worker or manager
nodes:

```
$docker swarm join-token  --rotate worker

To add a worker to this swarm, run the following command:
    docker swarm join \
    --token SWMTKN-1-2kscvs0zuymrsc9t0ocyy1rdns9dhaodvpl639j2bqx55uptag-ebmn5u927reawo27s3azntd44 \
    172.17.0.2:2377
```

## Learn More

* [Join nodes to a swarm](join-nodes.md)
* `swarm init`[command line reference](../reference/commandline/swarm_init.md)
* [Swarm mode tutorial](swarm-tutorial/index.md)

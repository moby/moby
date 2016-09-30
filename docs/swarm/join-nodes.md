<!--[metadata]>
+++
title = "Join nodes to a swarm"
description = "Add worker and manager nodes to a swarm"
keywords = ["guide, swarm mode, node"]
[menu.main]
identifier="join-nodes-guide"
parent="engine_swarm"
weight=13
+++
<![end-metadata]-->

# Join nodes to a swarm

When you first create a swarm, you place a single Docker Engine (Engine) into
swarm mode. To take full advantage of swarm mode you can add nodes to the swarm:

* Adding worker nodes increases capacity. When you deploy a service to a swarm,
the Engine schedules tasks on available nodes whether they are worker nodes or
manager nodes. When you add workers to your swarm, you increase the scale of
the swarm to handle tasks without affecting the manager raft consenus.
* Manager nodes increase fault-tolerance. Manager nodes perform the
orchestration and cluster management functions for the swarm. Among manager
nodes, a single leader node conducts orchestration tasks. If a leader node
goes down, the remaining manager nodes elect a new leader and resume
orchestration and maintenance of the swarm state. By default, manager nodes
also run tasks.

Before you add nodes to a swarm you must install Docker Engine 1.12 or later on
the host machine.

The Docker Engine joins the swarm depending on the **join-token** you provide to
the `docker swarm join` command. The node only uses the token at join time. If
you subsequently rotate the token, it doesn't affect existing swarm nodes. Refer
to [Run Docker Engine in swarm mode](swarm-mode.md#view-the-join-command-or-update-a-swarm-join-token).

## Join as a worker node

To retrieve the join command including the join token for worker nodes, run the
following command on a manager node:

```bash
$ docker swarm join-token worker

To add a worker to this swarm, run the following command:

    docker swarm join \
    --token SWMTKN-1-49nj1cmql0jkz5s954yi3oex3nedyz0fb0xx14ie39trti4wxv-8vxv8rssmk743ojnwacrr2e7c \
    192.168.99.100:2377
```

Run the command from the output on the worker to join the swarm:

```bash
$ docker swarm join \
  --token SWMTKN-1-49nj1cmql0jkz5s954yi3oex3nedyz0fb0xx14ie39trti4wxv-8vxv8rssmk743ojnwacrr2e7c \
  192.168.99.100:2377

This node joined a swarm as a worker.
```

The `docker swarm join` command does the following:

* switches the Docker Engine on the current node into swarm mode.
* requests a TLS certificate from the manager.
* names the node with the machine hostname
* joins the current node to the swarm at the manager listen address based upon the swarm token.
* sets the current node to `Active` availability, meaning it can receive tasks
from the scheduler.
* extends the `ingress` overlay network to the current node.

### Join as a manager node

When you run `docker swarm join` and pass the manager token, the Docker Engine
switches into swarm mode the same as for workers. Manager nodes also participate
in the raft consensus. The new nodes should be `Reachable`, but the existing
manager will remain the swarm `Leader`.

Docker recommends three or five manager nodes per cluster to implement high
availability. Because swarm mode manager nodes share data using Raft, there
must be an odd number of managers. The swarm can continue to function after as
long as a quorum of more than half of the manager nodes are available.

For more detail about swarm managers and administering a swarm, see
[Administer and maintain a swarm of Docker Engines](admin_guide.md).

To retrieve the join command including the join token for manager nodes, run the
following command on a manager node:

```bash
$ docker swarm join-token manager

To add a manager to this swarm, run the following command:

    docker swarm join \
    --token SWMTKN-1-61ztec5kyafptydic6jfc1i33t37flcl4nuipzcusor96k7kby-5vy9t8u35tuqm7vh67lrz9xp6 \
    192.168.99.100:2377
```

Run the command from the output on the manager to join the swarm:

```bash
$ docker swarm join \
  --token SWMTKN-1-61ztec5kyafptydic6jfc1i33t37flcl4nuipzcusor96k7kby-5vy9t8u35tuqm7vh67lrz9xp6 \
  192.168.99.100:2377

This node joined a swarm as a manager.
```

## Learn More

* `swarm join`[command line reference](../reference/commandline/swarm_join.md)
* [Swarm mode tutorial](swarm-tutorial/index.md)

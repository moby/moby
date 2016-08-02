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
a machine to serve as a manager node in your swarm.

If you haven't already, read through the [swarm mode key concepts](key-concepts.md)
and try the [swarm mode tutorial](swarm-tutorial/index.md).

## Create a swarm

When you run the command to create a swarm, the Docker Engine starts running in swarm mode.

Run [`docker swarm init`](../reference/commandline/swarm_init.md)
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
you join new worker nodes to the swarm:

```bash
$ docker swarm init
Swarm initialized: current node (dxn1zf6l61qsb1josjja83ngz) is now a manager.

To add a worker to this swarm, run the following command:

    docker swarm join \
    --token SWMTKN-1-49nj1cmql0jkz5s954yi3oex3nedyz0fb0xx14ie39trti4wxv-8vxv8rssmk743ojnwacrr2e7c \
    192.168.99.100:2377

To add a manager to this swarm, run 'docker swarm join-token manager' and follow the instructions.
```

### Configure the advertise address

Manager nodes use an advertise address to allow other nodes in the swarm access
to the Swarmkit API and overlay networking. The other nodes on the swarm must be
able to access the manager node on its advertise address IP address.

If you don't specify an advertise address, Docker checks if the system has a
single IP address. If so, Docker uses the IP address with with the listening
port `2377` by default. If the system has multiple IP addresses, you must
specify the correct  `--advertise-addr` to enable inter-manager communication
and overlay networking:

```bash
$ docker swarm init --advertise-addr <MANAGER-IP>
```

You must also specify the `--advertise-addr` if the address where other nodes
reach the first manager node is not the same address the manager sees as its
own. For instance, in a cloud setup that spans different regions, hosts have
both internal addresses for access within the region and external addresses that
you use for access from outside that region. In this case, specify the external
address with `--advertise-addr` so that the node can propogate that information
to other nodes that subsequently connect to it.

Refer to the `docker swarm init` [CLI reference](../reference/commandline/swarm_init.md)
for more detail on the advertise address.

### View the join command or update a swarm join token

Nodes require a secret token to join the swarm. The token for worker nodes is
different from the token for manager nodes. Nodes only use the join-token at the
moment they join the swarm. Rotating the join token after a node has already
joined a swarm does not affect the node's swarm membership. Token rotation
ensures an old token cannot be used by any new nodes attempting to join the
swarm.

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

Be careful with the join tokens because they are the secrets necessary to join
the swarm. In particular, checking a secret into version control is a bad
practice because it would allow anyone with access to the the application source
code to add new nodes to the swarm. Manager tokens are especially sensitive
because they allow a new manager node to join and gain control over the whole
swarm.

We recommend that you rotate the join tokens in the following circumstances:

* If a token was checked-in by accident into a version control system, group
chat or accidentally printed to your logs.
* If you suspect a node has been compromised.
* If you wish to guarantee that no new nodes can join the swarm.

Additionally, it is a best practice to implement a regular rotation schedule for
any secret including swarm join tokens. We recommend that you rotate your tokens
at least every 6 months.

Run `swarm join-token --rotate` to invalidate the old token and generate a new
token. Specify whether you want to rotate the token for `worker` or `manager`
nodes:

```bash
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

<!--[metadata]>
+++
aliases = [
"/engine/swarm/manager-administration-guide/"
]
title = "Swarm administration guide"
description = "Manager administration guide"
keywords = ["docker, container, cluster, swarm, manager, raft"]
[menu.main]
identifier="manager_admin_guide"
parent="engine_swarm"
weight="20"
+++
<![end-metadata]-->

# Administer and maintain a swarm of Docker Engines

When you run a swarm of Docker Engines, **manager nodes** are the key components
for managing the cluster and storing the cluster state. It is important to
understand some key features of manager nodes in order to properly deploy and
maintain the swarm.

This article covers the following swarm administration tasks:

* [Add Manager nodes for fault tolerance](#add-manager-nodes-for-fault-tolerance)
* [Distributing manager nodes](#distributing-manager-nodes)
* [Running manager-only nodes](#run-manager-only-nodes)
* [Backing up the cluster state](#back-up-the-cluster-state)
* [Monitoring the swarm health](#monitor-swarm-health)
* [Recovering from disaster](#recover-from-disaster)

Refer to [How swarm mode nodes work](how-swarm-mode-works/nodes.md)
for a brief overview of Docker Swarm mode and the difference between manager and
worker nodes.

## Operating manager nodes in a swarm

Swarm manager nodes use the [Raft Consensus Algorithm](raft.md) to manage the
cluster state. You only need to understand some general concepts of Raft in
order to manage a swarm.

There is no limit on the number of manager nodes. The decision about how many
manager nodes to implement is a trade-off between performance and
fault-tolerance. Adding manager nodes to a swarm makes the swarm more
fault-tolerant. However, additional manager nodes reduce write performance
because more nodes must acknowledge proposals to update the cluster state.
This means more network round-trip traffic.

Raft requires a majority of managers, also called a quorum, to agree on proposed
updates to the cluster. A quorum of managers must also agree on node additions
and removals. Membership operations are subject to the same constraints as state
replication.

## Add manager nodes for fault tolerance

You should maintain an odd number of managers in the swarm to support manager
node failures. Having an odd number of managers ensures that during a network
partition, there is a higher chance that a quorum remains available to process
requests if the network is partitioned into two sets. Keeping a quorum is not
guaranteed if you encounter more than two network partitions.

| Cluster Size |  Majority  |  Fault Tolerance  |
|:------------:|:----------:|:-----------------:|
|      1       |     1      |         0         |
|      2       |     2      |         0         |
|    **3**     |     2      |       **1**       |
|      4       |     3      |         2         |
|    **5**     |     3      |       **2**       |
|      6       |     4      |         2         |
|    **7**     |     4      |       **3**       |
|      8       |     5      |         3         |
|    **9**     |     5      |       **4**       |

For example, in a swarm with *5 nodes*, if you lose *3 nodes*, you don't have a
quorum. Therefore you can't add or remove nodes until you recover one of the
unavailable manager nodes or recover the cluster with disaster recovery
commands. See [Recover from disaster](#recover-from-disaster).

While it is possible to scale a swarm down to a single manager node, it is
impossible to demote the last manager node. This ensures you maintain access to
the swarm and that the swarm can still process requests. Scaling down to a
single manager is an unsafe operation and is not recommended. If
the last node leaves the cluster unexpetedly during the demote operation, the
cluster swarm will become unavailable until you reboot the node or restart with
`--force-new-cluster`.

You manage cluster membership with the `docker swarm` and `docker node`
subsystems. Refer to [Add nodes to a swarm](join-nodes.md) for more information
on how to add worker nodes and promote a worker node to be a manager.

## Distributing manager nodes

In addition to maintaining an odd number of manager nodes, pay attention to
datacenter topology when placing managers. For optimal fault-tolerance, distribute
manager nodes across a minimum of 3 availability-zones to support failures of an
entire set of machines or common maintenance scenarios. If you suffer a failure
in any of those zones, the swarm should maintain a quorum of manager nodes
available to process requests and rebalance workloads.

| Swarm manager nodes |  Repartition (on 3 Availability zones) |
|:-------------------:|:--------------------------------------:|
| 3                   |                  1-1-1                 |
| 5                   |                  2-2-1                 |
| 7                   |                  3-2-2                 |
| 9                   |                  3-3-3                 |

## Run manager-only nodes

By default manager nodes also act as a worker nodes. This means the scheduler
can assign tasks to a manager node. For small and non-critical clusters
assigning tasks to managers is relatively low-risk as long as you schedule
services using **resource constraints** for *cpu* and *memory*.

However, because manager nodes use the Raft consensus algorithm to replicate data
in a consistent way, they are sensitive to resource starvation. You should
isolate managers in your swarm from processes that might block cluster
operations like cluster heartbeat or leader elections.

To avoid interference with manager node operation, you can drain manager nodes
to make them unavailable as worker nodes:

```bash
docker node update --availability drain <NODE-ID>
```

When you drain a node, the scheduler reassigns any tasks running on the node to
other available worker nodes in the cluster. It also prevents the scheduler from
assigning tasks to the node.

## Back up the cluster state

Docker manager nodes store the cluster state and manager logs in the following
directory:

`/var/lib/docker/swarm/raft`

Back up the raft data directory often so that you can use it in case of disaster
recovery.

You should never restart a manager node with the data directory from another
node (for example, by copying the `raft` directory from one node to another).
The data directory is unique to a node ID and a node can only use a given node
ID once to join the swarm. (ie. Node ID space should be globally unique)

To cleanly re-join a manager node to a cluster:

1. Run `docker node demote <id-node>` to demote the node to a worker.
2. Run `docker node rm <id-node>` before adding a node back with a fresh state.
3. Re-join the node to the cluster using `docker swarm join`.

In case of [disaster recovery](#recover-from-disaster), you can take the raft data
directory of one of the manager nodes to restore to a new swarm cluster.

## Monitor swarm health

You can monitor the health of Manager nodes by querying the docker `nodes` API
in JSON format through the `/nodes` HTTP endpoint. Refer to the [nodes API documentation](../reference/api/docker_remote_api_v1.24.md#36-nodes)
for more information.

From the command line, run `docker node inspect <id-node>` to query the nodes.
For instance, to query the reachability of the node as a Manager:

```bash
docker node inspect manager1 --format "{{ .ManagerStatus.Reachability }}"
reachable
```

To query the status of the node as a Worker that accept tasks:

```bash
docker node inspect manager1 --format "{{ .Status.State }}"
ready
```

From those commands, we can see that `manager1` is both at the status
`reachable` as a manager and `ready` as a worker.

An `unreachable` health status means that this particular manager node is unreachable
from other manager nodes. In this case you need to take action to restore the unreachable
manager:

- Restart the daemon and see if the manager comes back as reachable.
- Reboot the machine.
- If neither restarting or rebooting work, you should add another manager node or promote a worker to be a manager node. You also need to cleanly remove the failed node entry from the Manager set with `docker node demote <id-node>` and `docker node rm <id-node>`.

Alternatively you can also get an overview of the cluster health with `docker node ls`:

```bash
# From a Manager node
docker node ls
ID                           HOSTNAME  MEMBERSHIP  STATUS  AVAILABILITY  MANAGER STATUS
1mhtdwhvsgr3c26xxbnzdc3yp    node05    Accepted    Ready   Active
516pacagkqp2xc3fk9t1dhjor    node02    Accepted    Ready   Active        Reachable
9ifojw8of78kkusuc4a6c23fx *  node01    Accepted    Ready   Active        Leader
ax11wdpwrrb6db3mfjydscgk7    node04    Accepted    Ready   Active
bb1nrq2cswhtbg4mrsqnlx1ck    node03    Accepted    Ready   Active        Reachable
di9wxgz8dtuh9d2hn089ecqkf    node06    Accepted    Ready   Active
```

## Manager advertise address

When initiating or joining a Swarm cluster, you have to specify the `--listen-addr`
flag to advertise your address to other Manager nodes in the cluster.

We recommend that you use a *fixed IP address* for the advertised address, otherwise
the cluster could become unstable on machine reboot.

Indeed if the whole cluster restarts and every Manager gets a new IP address on
restart, there is no way for any of those nodes to contact an existing Manager
and the cluster will stay stuck trying to contact other nodes through their old address.
While having dynamic IP addresses for Worker nodes is acceptable, Managers are
meant to be a stable piece in the infrastructure thus it is highly recommended to
deploy those critical nodes with static IPs.

## Recover from disaster

Swarm is resilient to failures and the cluster can recover from any number
of temporary node failures (machine reboots or crash with restart).

In a swarm of `N` managers, there must be a quorum of manager nodes greater than
50% of the total number of managers (or `(N/2)+1`) in order for the swarm to
process requests and remain available. This means the swarm can tolerate up to
`(N-1)/2` permanent failures beyond which requests involving cluster management
cannot be processed. These types of failures include data corruption or hardware
failures.

Even if you follow the guidelines here, it is possible that you can lose a
quorum of manager nodes. If you can't recover the quorum by conventional
means such as restarting faulty nodes, you can recover the cluster by running
`docker swarm init --force-new-cluster` on a manager node.

```bash
# From the node to recover
docker swarm init --force-new-cluster --listen-addr node01:2377
```

The `--force-new-cluster` flag puts the Docker Engine into swarm mode as a
manager node of a single-node cluster. It discards cluster membership information
that existed before the loss of the quorum but it retains data necessary to the
Swarm cluster such as services, tasks and the list of worker nodes.

<!--[metadata]>
+++
title = "Raft consensus in swarm mode"
description = "Raft consensus algorithm in swarm mode"
keywords = ["docker, container, cluster, swarm, raft"]
[menu.main]
identifier="raft"
parent="engine_swarm"
weight="21"
+++
<![end-metadata]-->

## Raft consensus algorithm

When the Docker Engine runs in swarm mode, manager nodes implement the
[Raft Consensus Algorithm](http://thesecretlivesofdata.com/raft/) to manage the global cluster state.

The reason why *Docker swarm mode* is using a consensus algorithm is to make sure that
all the manager nodes that are in charge of managing and scheduling tasks in the cluster,
are storing the same consistent state.

Having the same consistent state across the cluster means that in case of a failure,
any Manager node can pick up the tasks and restore the services to a stable state.
For example, if the *Leader Manager* which is responsible for scheduling tasks in the
cluster dies unexpectedly, any other Manager can pick up the task of scheduling and
re-balance tasks to match the desired state.

Systems using consensus algorithms to replicate logs in a distributed systems
do require special care. They ensure that the cluster state stays consistent
in the presence of failures by requiring a majority of nodes to agree on values.

Raft tolerates up to `(N-1)/2` failures and requires a majority or quorum of
`(N/2)+1` members to agree on values proposed to the cluster. This means that in
a cluster of 5 Managers running Raft, if 3 nodes are unavailable, the system
will not process any more requests to schedule additional tasks. The existing
tasks will keep running but the scheduler will not be able to rebalance tasks to
cope with failures if when the manager set is not healthy.

The implementation of the consensus algorithm in swarm mode means it features
the properties inherent to distributed systems:

- *agreement on values* in a fault tolerant system. (Refer to [FLP impossibility theorem](http://the-paper-trail.org/blog/a-brief-tour-of-flp-impossibility/)
 and the [Raft Consensus Algorithm paper](https://www.usenix.org/system/files/conference/atc14/atc14-paper-ongaro.pdf))
- *mutual exclusion* through the leader election process
- *cluster membership* management
- *globally consistent object sequencing* and CAS (compare-and-swap) primitives

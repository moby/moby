<!--[metadata]>
+++
title = "swarm init"
description = "The swarm init command description and usage"
keywords = ["swarm, init"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

**Warning:** this command is part of the Swarm management feature introduced in Docker 1.12, and might be subject to non backward-compatible changes.

# swarm init

	Usage:	docker swarm init [OPTIONS]

	Initialize a Swarm.

	Options:
	      --auto-accept value   Acceptance policy (default [worker,manager])
	      --force-new-cluster   Force create a new cluster from current state.
	      --help                Print usage
	      --listen-addr value   Listen address (default 0.0.0.0:2377)
	      --secret string       Set secret value needed to accept nodes into cluster

Initialize a Swarm cluster. The docker engine targeted by this command becomes a manager
in the newly created one node Swarm cluster.


```bash
$ docker swarm init --listen-addr 192.168.99.121:2377
Initializing a new swarm
$ docker node ls
ID              NAME          STATUS  AVAILABILITY/MEMBERSHIP  MANAGER STATUS  LEADER
3l1f6uzcuoa3 *  swarm-master  READY   ACTIVE                   REACHABLE       Yes
```

###	--auto-accept value

This flag controls node acceptance into the cluster. By default, both `worker` and `manager`
nodes are auto accepted by the cluster. This can be changed by specifing what kinds of nodes
can be auto-accepted into the cluster. If auto-accept is not turned on, then 
[node accept](node_accept.md) can be used to explicitly accept a node into the cluster.

For example, the following initializes a cluster with auto-acceptance of workers, but not managers


```bash
$ docker swarm init --listen-addr 192.168.99.121:2377 --auto-accept worker
Initializing a new swarm
```

### `--force-new-cluster`

This flag forces an existing node that was part of a quorum that was lost to restart as a single node Manager without losing its data

### `--listen-addr value`

The node listens for inbound Swarm manager traffic on this IP:PORT

### `--secret string`

Secret value needed to accept nodes into the Swarm

## Related information

* [swarm join](swarm_join.md)
* [swarm leave](swarm_leave.md)
* [swarm update](swarm_update.md)

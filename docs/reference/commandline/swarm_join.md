<!--[metadata]>
+++
title = "swarm join"
description = "The swarm join command description and usage"
keywords = ["swarm, join"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

**Warning:** this command is part of the Swarm management feature introduced in Docker 1.12, and might be subject to non backward-compatible changes.

# swarm join

	Usage:	docker swarm join [OPTIONS] HOST:PORT

	Join a Swarm as a node and/or manager.

	Options:
	      --help                Print usage
	      --listen-addr value   Listen address (default 0.0.0.0:2377)
	      --manager             Try joining as a manager.
	      --secret string       Secret for node acceptance

Join a node to a Swarm cluster. If the `--manager` flag is specified, the docker engine
targeted by this command becomes a `manager`. If it is not specified, it becomes a `worker`.

### Join a node to swarm as a manager

```bash
$ docker swarm join --manager --listen-addr 192.168.99.122:2377 192.168.99.121:2377
This node is attempting to join a Swarm as a manager.
$ docker node ls
ID              NAME           STATUS  AVAILABILITY/MEMBERSHIP  MANAGER STATUS  LEADER
2fg70txcrde2    swarm-node-01  READY   ACTIVE                   REACHABLE       
3l1f6uzcuoa3 *  swarm-master   READY   ACTIVE                   REACHABLE       Yes
```

### Join a node to swarm as a worker

```bash
$ docker swarm join --listen-addr 192.168.99.123:2377 192.168.99.121:2377
This node is attempting to join a Swarm.
$ docker node ls
ID              NAME           STATUS  AVAILABILITY/MEMBERSHIP  MANAGER STATUS  LEADER
04zm7ue1fd1q    swarm-node-02  READY   ACTIVE                                   
2fg70txcrde2    swarm-node-01  READY   ACTIVE                   REACHABLE       
3l1f6uzcuoa3 *  swarm-master   READY   ACTIVE                   REACHABLE       Yes
```

### `--manager`

Joins the node as a manager

### `--listen-addr value`

The node listens for inbound Swarm manager traffic on this IP:PORT

### `--secret string`

Secret value required for nodes to join the swarm


## Related information

* [swarm init](swarm_init.md)
* [swarm leave](swarm_leave.md)
* [swarm update](swarm_update.md)

<!--[metadata]>
+++
title = "swarm join"
description = "The swarm join command description and usage"
keywords = ["swarm, join"]
advisory = "rc"
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

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
This node joined a Swarm as a manager.
$ docker node ls
ID                           NAME      MEMBERSHIP  STATUS  AVAILABILITY  MANAGER STATUS         LEADER
dkp8vy1dq1kxleu9g4u78tlag *  manager2  Accepted    Ready   Active        Reachable
dvfxp4zseq4s0rih1selh0d20    manager1  Accepted    Ready   Active        Reachable              Yes
```

### Join a node to swarm as a worker

```bash
$ docker swarm join --listen-addr 192.168.99.123:2377 192.168.99.121:2377
This node joined a Swarm as a worker.
$ docker node ls
ID                           NAME      MEMBERSHIP  STATUS  AVAILABILITY  MANAGER STATUS         LEADER
7ln70fl22uw2dvjn2ft53m3q5    worker2   Accepted    Ready   Active
dkp8vy1dq1kxleu9g4u78tlag    worker1   Accepted    Ready   Active        Reachable
dvfxp4zseq4s0rih1selh0d20 *  manager1  Accepted    Ready   Active        Reachable              Yes
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

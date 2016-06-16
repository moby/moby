<!--[metadata]>
+++
title = "swarm leave"
description = "The swarm leave command description and usage"
keywords = ["swarm, leave"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

**Warning:** this command is part of the Swarm management feature introduced in Docker 1.12, and might be subject to non backward-compatible changes.

# swarm leave

	Usage:	docker swarm leave

	Leave a Swarm swarm.

	Options:
	      --help   Print usage

This command causes the node to leave the swarm.

On a manager node:
```bash
$ docker node ls
ID              NAME           STATUS  AVAILABILITY/MEMBERSHIP  MANAGER STATUS  LEADER
04zm7ue1fd1q    swarm-node-02  READY   ACTIVE                                   
2fg70txcrde2    swarm-node-01  READY   ACTIVE                   REACHABLE       
3l1f6uzcuoa3 *  swarm-master   READY   ACTIVE                   REACHABLE       Yes
```

On a worker node:
```bash
$ docker swarm leave
Node left the default swarm.
```

On a manager node:
```bash
$ docker node ls
ID              NAME           STATUS  AVAILABILITY/MEMBERSHIP  MANAGER STATUS  LEADER
04zm7ue1fd1q    swarm-node-02  DOWN    ACTIVE                                   
2fg70txcrde2    swarm-node-01  READY   ACTIVE                   REACHABLE       
3l1f6uzcuoa3 *  swarm-master   READY   ACTIVE                   REACHABLE       Yes
```

## Related information

* [swarm init](swarm_init.md)
* [swarm join](swarm_join.md)
* [swarm update](swarm_update.md)

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

```markdown
Usage:  docker swarm join [OPTIONS] HOST:PORT

Join a swarm as a node and/or manager

Options:
      --help                Print usage
      --listen-addr value   Listen address (default 0.0.0.0:2377)
      --token string        Token for entry into the swarm
```

Join a node to a swarm. The node joins as a manager node or worker node based upon the token you
pass with the `--token` flag. If you pass a manager token, the node joins as a manager. If you
pass a worker token, the node joins as a worker.

### Join a node to swarm as a manager

The example below demonstrates joining a manager node using a manager token.

```bash
$ docker swarm join --token SWMTKN-1-3pu6hszjas19xyp7ghgosyx9k8atbfcr8p2is99znpy26u2lkl-7p73s1dx5in4tatdymyhg9hu2 --listen-addr 192.168.99.122:2377 192.168.99.121:2377
This node joined a swarm as a manager.
$ docker node ls
ID                           HOSTNAME  STATUS  AVAILABILITY  MANAGER STATUS
dkp8vy1dq1kxleu9g4u78tlag *  manager2  Ready   Active        Reachable
dvfxp4zseq4s0rih1selh0d20    manager1  Ready   Active        Leader
```

A cluster should only have 3-7 managers at most, because a majority of managers must be available
for the cluster to function. Nodes that aren't meant to participate in this management quorum
should join as workers instead. Managers should be stable hosts that have static IP addresses.

### Join a node to swarm as a worker

The example below demonstrates joining a worker node using a worker token.

```bash
$ docker swarm join --token SWMTKN-1-3pu6hszjas19xyp7ghgosyx9k8atbfcr8p2is99znpy26u2lkl-1awxwuwd3z9j1z3puu7rcgdbx --listen-addr 192.168.99.123:2377 192.168.99.121:2377
This node joined a swarm as a worker.
$ docker node ls
ID                           HOSTNAME  STATUS  AVAILABILITY  MANAGER STATUS
7ln70fl22uw2dvjn2ft53m3q5    worker2   Ready   Active
dkp8vy1dq1kxleu9g4u78tlag    worker1   Ready   Active        Reachable
dvfxp4zseq4s0rih1selh0d20 *  manager1  Ready   Active        Leader
```

### `--listen-addr value`

The node listens for inbound swarm manager traffic on this IP:PORT

### `--token string`

Secret value required for nodes to join the swarm


## Related information

* [swarm init](swarm_init.md)
* [swarm leave](swarm_leave.md)
* [swarm update](swarm_update.md)

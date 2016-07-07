<!--[metadata]>
+++
title = "swarm init"
description = "The swarm init command description and usage"
keywords = ["swarm, init"]
advisory = "rc"
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# swarm init

	Usage:	docker swarm init [OPTIONS]

	Initialize a Swarm.

	Options:
	      --auto-accept value   Acceptance policy (default [worker,manager])
	      --external-ca value   Specifications of one or more certificate signing endpoints
	      --force-new-cluster   Force create a new cluster from current state.
	      --help                Print usage
	      --listen-addr value   Listen address (default 0.0.0.0:2377)
	      --secret string       Set secret value needed to accept nodes into cluster

Initialize a Swarm cluster. The docker engine targeted by this command becomes a manager
in the newly created one node Swarm cluster.


```bash
$ docker swarm init --listen-addr 192.168.99.121:2377
No --secret provided. Generated random secret:
	4ao565v9jsuogtq5t8s379ulb

Swarm initialized: current node (1ujecd0j9n3ro9i6628smdmth) is now a manager.

To add a worker to this swarm, run the following command:
	docker swarm join --secret 4ao565v9jsuogtq5t8s379ulb \
	--ca-hash sha256:07ce22bd1a7619f2adc0d63bd110479a170e7c4e69df05b67a1aa2705c88ef09 \
	192.168.99.121:2377
$ docker node ls
ID                           HOSTNAME  MEMBERSHIP  STATUS  AVAILABILITY  MANAGER STATUS          LEADER
1ujecd0j9n3ro9i6628smdmth *  manager1  Accepted    Ready   Active        Reachable               Yes
```

If a secret for joining new nodes is not provided with `--secret`, `docker swarm init` will
generate a random one and print it to the terminal (as seen in the example above). To initialize
a swarm with no secret, use `--secret ""`.

### `--auto-accept value`

This flag controls node acceptance into the cluster. By default, `worker` nodes are
automatically accepted by the cluster. This can be changed by specifying what kinds of nodes
can be auto-accepted into the cluster. If auto-accept is not turned on, then
[node accept](node_accept.md) can be used to explicitly accept a node into the cluster.

For example, the following initializes a cluster with auto-acceptance of workers, but not managers


```bash
$ docker swarm init --listen-addr 192.168.99.121:2377 --auto-accept worker
```

### `--external-ca value`

This flag sets up the swarm to use an external CA to issue node certificates. The value takes
the form `protocol=X,url=Y`. The value for `protocol` specifies what protocol should be used
to send signing requests to the external CA. Currently, the only supported value is `cfssl`.
The URL specifies the endpoint where signing requests should be submitted.

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

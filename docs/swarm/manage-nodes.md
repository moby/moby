<!--[metadata]>
+++
title = "Manage nodes in a swarm"
description = "Manage existing nodes in a swarm"
keywords = ["guide, swarm mode, node"]
[menu.main]
identifier="manage-nodes-guide"
parent="engine_swarm"
weight=14
+++
<![end-metadata]-->

# Manage nodes in a swarm

As part of the swarm management lifecycle, you may need to view or update a node as follows:

* [list nodes in the swarm](#list-nodes)
* [inspect an individual node](#inspect-an-individual-node)
* [update a node](#update-a-node)
* [leave the swarm](#leave-the-swarm)

## List nodes

To view a list of nodes in the swarm run `docker node ls` from a manager node:

```bash
$ docker node ls

ID                           HOSTNAME  STATUS  AVAILABILITY  MANAGER STATUS
46aqrk4e473hjbt745z53cr3t    node-5    Ready   Active        Reachable
61pi3d91s0w3b90ijw3deeb2q    node-4    Ready   Active        Reachable
a5b2m3oghd48m8eu391pefq5u    node-3    Ready   Active
e7p8btxeu3ioshyuj6lxiv6g0    node-2    Ready   Active
ehkv3bcimagdese79dn78otj5 *  node-1    Ready   Active        Leader
```

The `AVAILABILITY` column shows whether or not the scheduler can assign tasks to
the node:

* `Active` means that the scheduler can assign tasks to a node.
* `Pause` means the scheduler doesn't assign new tasks to the node, but existing
tasks remain running.
* `Drain` means the scheduler doesn't assign new tasks to the node. The
scheduler shuts down any existing tasks and schedules them on an available
node.

The `MANAGER STATUS` column shows node participation in the Raft consensus:

* No value indicates a worker node that does not participate in swarm
management.
* `Leader` means the node is the primary manager node that makes all swarm
management and orchestration decisions for the swarm.
* `Reachable` means the node is a manager node is participating in the Raft
consensus. If the leader node becomes unavailable, the node is eligible for
election as the new leader.
* `Unavailable` means the node is a manager that is not able to communicate with
other managers. If a manager node becomes unavailable, you should either join a
new manager node to the swarm or promote a worker node to be a
manager.

For more information on swarm administration refer to the [Swarm administration guide](admin_guide.md).

## Inspect an individual node

You can run `docker node inspect <NODE-ID>` on a manager node to view the
details for an individual node. The output defaults to JSON format, but you can
pass the `--pretty` flag to print the results in human-readable format. For example:

```bash
docker node inspect self --pretty

ID:                     ehkv3bcimagdese79dn78otj5
Hostname:               node-1
Joined at:              2016-06-16 22:52:44.9910662 +0000 utc
Status:
 State:                 Ready
 Availability:          Active
Manager Status:
 Address:               172.17.0.2:2377
 Raft Status:           Reachable
 Leader:                Yes
Platform:
 Operating System:      linux
 Architecture:          x86_64
Resources:
 CPUs:                  2
 Memory:                1.954 GiB
Plugins:
  Network:              overlay, host, bridge, overlay, null
  Volume:               local
Engine Version:         1.12.0-dev
```

## Update a node

You can modify node attributes as follows:

* [change node availability](#change-node-availability)
* [add or remove label metadata](#add-or-remove-label-metadata)
* [change a node role](#promote-or-demote-a-node)

### Change node availability

Changing node availability lets you:

* drain a manager node so that only performs swarm management tasks and is
unavailable for task assignment.
* drain a node so you can take it down for maintenance.
* pause a node so it is unavailable to receive new tasks.
* restore unavailable or paused nodes available status.

For example, to change a manager node to `Drain` availability:

```bash
$ docker node update --availability drain node-1

node-1
```

See [list nodes](#list-nodes) for descriptions of the different availability
options.

### Add or remove label metadata

Node labels provide a flexible method of node organization. You can also use
node labels in service constraints. Apply constraints when you create a service
to limit the nodes where the scheduler assigns tasks for the service.

Run `docker node update --label-add` on a manager node to add label metadata to
a node. The `--label-add` flag supports either a `<key>` or a `<key>=<value>`
pair.

Pass the `--label-add` flag once for each node label you want to add:

```bash
$ docker node update --label-add foo --label-add bar=baz node-1

node-1
```

The labels you set for nodes using docker node update apply only to the node
entity within the swarm. Do not confuse them with the docker daemon labels for
[dockerd](../userguide/labels-custom-metadata.md#daemon-labels).

Refer to the `docker service create` [CLI reference](../reference/commandline/service_create.md)
for more information about service constraints.

### Promote or demote a node

You can promote a worker node to the manager role. This is useful when a
manager node becomes unavailable or if you want to take a manager offline for
maintenance. Similarly, you can demote a manager node to the worker role.

Regardless of your reason to promote or demote a node, you should always
maintain an odd number of manager nodes in the swarm. For more information refer
to the [Swarm administration guide](admin_guide.md).

To promote a node or set of nodes, run `docker node promote` from a manager
node:

```bash
$ docker node promote node-3 node-2

Node node-3 promoted to a manager in the swarm.
Node node-2 promoted to a manager in the swarm.
```

To demote a node or set of nodes, run `docker node demote` from a manager node:

```bash
$ docker node demote node-3 node-2

Manager node-3 demoted in the swarm.
Manager node-2 demoted in the swarm.
```

`docker node promote` and `docker node demote` are convenience commands for
`docker node update --role manager` and `docker node update --role worker`
respectively.


## Leave the swarm

Run the `docker swarm leave` command on a node to remove it from the swarm.

For example to leave the swarm on a worker node:

```bash
$ docker swarm leave

Node left the swarm.
```

When a node leaves the swarm, the Docker Engine stops running in swarm
mode. The orchestrator no longer schedules tasks to the node.

If the node is a manager node, you will receive a warning about maintaining the
quorum. To override the warning, pass the `--force` flag. If the last manager
node leaves the swarm, the swarm becomes unavailable requiring you to take
disaster recovery measures.

For information about maintaining a quorum and disaster recovery, refer to the
[Swarm administration guide](admin_guide.md).

After a node leaves the swarm, you can run the `docker node rm` command on a
manager node to remove the node from the node list.

For instance:

```bash
docker node rm node-2

node-2
```

## Learn More

* [Swarm administration guide](admin_guide.md)
* [Docker Engine command line reference](../reference/commandline/index.md)
* [Swarm mode tutorial](swarm-tutorial/index.md)

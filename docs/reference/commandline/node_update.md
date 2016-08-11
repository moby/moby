<!--[metadata]>
+++
title = "node update"
description = "The node update command description and usage"
keywords = ["resources, update, dynamically"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

## update

```markdown
Usage:  docker node update [OPTIONS] NODE

Update a node

Options:
      --availability string   Availability of the node (active/pause/drain)
      --help                  Print usage
      --label-add value       Add or update a node label (key=value) (default [])
      --label-rm value        Remove a node label if exists (default [])
      --role string           Role of the node (worker/manager)
```

This command is a *low-level* command to update membership, role and
availability of a node. Each of these attributes can be updated using
a simpler command such as `accept` for membership, `promote` or
`demote` for role and `activate`, `drain` or `pause` for
availability.

This command must be run on a manager node, but may update any node in
the swarm.

### Add label metadata to a node

Add metadata to a swarm node using node labels. You can specify a node label as
a key with an empty value:

``` bash
$ docker node update --label-add foo worker1
```

To add multiple labels to a node, pass the `--label-add` flag for each label:

``` bash
$ docker node update --label-add foo --label-add bar worker1
```

When you [create a service](service_create.md),
you can use node labels as a constraint. A constraint limits the nodes where the
scheduler deploys tasks for a service.

For example, to add a `type` label to identify nodes where the scheduler should
deploy message queue service tasks:

``` bash
$ docker node update --label-add type=queue worker1
```

The labels you set for nodes using `docker node update` apply only to the node
entity within the swarm. Do not confuse them with the docker daemon labels for
[dockerd]( ../../userguide/labels-custom-metadata.md#daemon-labels).

For more information about labels, refer to [apply custom
metadata](../../userguide/labels-custom-metadata.md).


## Related information

* [node accept](node_accept.md)
* [node active](node_activate.md)
* [node demote](node_demote.md)
* [node drain](node_drain.md)
* [node inspect](node_inspect.md)
* [node ls](node_ls.md)
* [node pause](node_pause.md)
* [node promote](node_promote.md)
* [node rm](node_rm.md)
* [node tasks](node_tasks.md)

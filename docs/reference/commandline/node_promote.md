<!--[metadata]>
+++
title = "node promote"
description = "The node promote command description and usage"
keywords = ["node, promote"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# node promote

```markdown
Usage:  docker node promote NODE [NODE...]

Promote one or more nodes to manager in the swarm

Options:
      --help   Print usage
```

Promotes a worker node to a manager. Becoming a manager means the node
will be able to accept administrative commands for the swarm, and
become one of the candidates when the swarm elects a leader. The node
will continue to be assigned tasks unless it is paused with `docker
node pause` or drained with `docker node drain`. Only specific, stable
nodes should be promoted to managers, because a majority of leaders
must be responsive for the swarm to be available. Generally, a swarm
should be set up with either 1, 3, or 5 managers.

This command must be run on a manager node, but may promote any node
in the swarm.

```bash
$ docker node promote <node name>
```

## Related information

* [node accept](node_accept.md)
* [node active](node_activate.md)
* [node demote](node_demote.md)
* [node drain](node_drain.md)
* [node inspect](node_inspect.md)
* [node ls](node_ls.md)
* [node pause](node_pause.md)
* [node rm](node_rm.md)
* [node tasks](node_tasks.md)
* [node update](node_update.md)

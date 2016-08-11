<!--[metadata]>
+++
title = "node drain"
description = "The node drain command description and usage"
keywords = ["node, drain"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# node drain

    Usage:  docker node drain NODE [NODE...]

    Drain a node in the swarm

This command marks a node as drained so that all tasks will be removed
from this node, and no tasks will be assigned to it until it is
reactivated (using `docker node activate`).

This command must be run on a manager node, but may drain any node in
the swarm.

```bash
$ docker node drain <node name>
```

## Related information

* [node accept](node_accept.md)
* [node active](node_activate.md)
* [node demote](node_demote.md)
* [node inspect](node_inspect.md)
* [node ls](node_ls.md)
* [node pause](node_pause.md)
* [node promote](node_promote.md)
* [node rm](node_rm.md)
* [node tasks](node_tasks.md)
* [node update](node_update.md)

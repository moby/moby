<!--[metadata]>
+++
title = "node pause"
description = "The node pause command description and usage"
keywords = ["node, pause"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# node pause

    Usage:  docker node pause NODE [NODE...]

    Pause a node in the swarm

This command marks a node as paused so that it doesn't accept any new
tasks until it's been reactivated (using `docker node activate`).

This command must be run on a manager node, but may pause any node in
the swarm.

```bash
$ docker node pause <node name>
```

## Related information

* [node accept](node_accept.md)
* [node active](node_activate.md)
* [node demote](node_demote.md)
* [node drain](node_drain.md)
* [node inspect](node_inspect.md)
* [node ls](node_ls.md)
* [node promote](node_promote.md)
* [node rm](node_rm.md)
* [node tasks](node_tasks.md)
* [node update](node_update.md)

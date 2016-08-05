<!--[metadata]>
+++
title = "node activate"
description = "The node activate command description and usage"
keywords = ["node, activate"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# node activate

    Usage:  docker node activate NODE [NODE...]

    Activate a node in the swarm

This command marks a previously drained or paused node as active again
so that it can start accepting tasks again.

This command must be run on a manager node, but may activate any node
in the swarm.

```bash
$ docker node activate <node name>
```

## Related information

* [node accept](node_accept.md)
* [node demote](node_demote.md)
* [node drain](node_drain.md)
* [node inspect](node_inspect.md)
* [node ls](node_ls.md)
* [node pause](node_pause.md)
* [node promote](node_promote.md)
* [node rm](node_rm.md)
* [node tasks](node_tasks.md)
* [node update](node_update.md)

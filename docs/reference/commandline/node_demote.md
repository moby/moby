<!--[metadata]>
+++
title = "node demote"
description = "The node demote command description and usage"
keywords = ["node, demote"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# node demote

```markdown
Usage:  docker node demote NODE [NODE...]

Demote one or more nodes from manager in the swarm

Options:
      --help   Print usage

```

Demotes an existing manager so that it is no longer a manager.

This command must be run on a manager node, but may demote any node in
the swarm.


```bash
$ docker node demote <node name>
```

## Related information

* [node accept](node_accept.md)
* [node active](node_activate.md)
* [node drain](node_drain.md)
* [node inspect](node_inspect.md)
* [node ls](node_ls.md)
* [node pause](node_pause.md)
* [node promote](node_promote.md)
* [node rm](node_rm.md)
* [node tasks](node_tasks.md)
* [node update](node_update.md)

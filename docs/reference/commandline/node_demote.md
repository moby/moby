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

    Usage:  docker node demote NODE [NODE...]

    Demote a node as manager in the swarm

Demotes an existing Manager so that it is no longer a manager. This command targets a docker engine that is a manager in the swarm cluster.


```bash
$ docker node demote <node name>
```

## Related information

* [node accept](node_accept.md)
* [node reject](node_reject.md)
* [node promote](node_promote.md)

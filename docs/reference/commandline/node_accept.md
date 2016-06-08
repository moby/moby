<!--[metadata]>
+++
title = "node accept"
description = "The node accept command description and usage"
keywords = ["node, accept"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# node accept

    Usage:  docker node accept NODE [NODE...]

    Accept a node in the swarm

Accept a node into the swarm. This command targets a docker engine that is a manager in the swarm cluster.


```bash
$ docker node accept <node name>
```

## Related information

* [node reject](node_reject.md)
* [node promote](node_promote.md)
* [node demote](node_demote.md)

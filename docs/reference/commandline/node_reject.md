<!--[metadata]>
+++
title = "node reject"
description = "The node reject command description and usage"
keywords = ["node, reject"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# node reject

    Usage:  docker node reject NODE [NODE...]

    Reject a node from the swarm

Reject a node from joining the swarm. This command targets a docker engine that is a manager in the swarm cluster.


```bash
$ docker node reject <node name>
```

## Related information

* [node accept](node_accept.md)
* [node promote](node_promote.md)
* [node demote](node_demote.md)

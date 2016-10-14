---
title: "node demote"
description: "The node demote command description and usage"
keywords: ["node, demote"]
---

# node demote

```markdown
Usage:  docker node demote NODE [NODE...]

Demote one or more nodes from manager in the swarm

Options:
      --help   Print usage

```

Demotes an existing manager so that it is no longer a manager. This command targets a docker engine that is a manager in the swarm.


```bash
$ docker node demote <node name>
```

## Related information

* [node promote](node_promote.md)

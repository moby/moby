---
title: "node promote"
description: "The node promote command description and usage"
keywords: ["node, promote"]
---

# node promote

```markdown
Usage:  docker node promote NODE [NODE...]

Promote one or more nodes to manager in the swarm

Options:
      --help   Print usage
```

Promotes a node to manager. This command targets a docker engine that is a manager in the swarm.


```bash
$ docker node promote <node name>
```

## Related information

* [node demote](node_demote.md)

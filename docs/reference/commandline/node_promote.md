---
title: "node promote"
description: "The node promote command description and usage"
keywords: "node, promote"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# node promote

```markdown
Usage:  docker node promote NODE [NODE...]

Promote one or more nodes to manager in the swarm

Options:
      --help   Print usage
```

## Description

Promotes a node to manager. This command targets a docker engine that is a
manager in the swarm.

## Examples

```bash
$ docker node promote <node name>
```

## Related commands

* [node demote](node_demote.md)
* [node inspect](node_inspect.md)
* [node ls](node_ls.md)
* [node ps](node_ps.md)
* [node rm](node_rm.md)
* [node update](node_update.md)

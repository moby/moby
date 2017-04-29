---
title: "node demote"
description: "The node demote command description and usage"
keywords: "node, demote"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# node demote

```markdown
Usage:  docker node demote NODE [NODE...]

Demote one or more nodes from manager in the swarm

Options:
      --help   Print usage

```

## Description

Demotes an existing manager so that it is no longer a manager. This command
targets a docker engine that is a manager in the swarm.


## Examples

```bash
$ docker node demote <node name>
```

## Related commands

* [node inspect](node_inspect.md)
* [node ls](node_ls.md)
* [node promote](node_promote.md)
* [node ps](node_ps.md)
* [node rm](node_rm.md)
* [node update](node_update.md)

---
title: "swarm leave"
description: "The swarm leave command description and usage"
keywords: "swarm, leave"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# swarm leave

```markdown
Usage:	docker swarm leave [OPTIONS]

Leave the swarm

Options:
  -f, --force   Force this node to leave the swarm, ignoring warnings
      --help    Print usage
```

## Description

When you run this command on a worker, that worker leaves the swarm.

You can use the `--force` option on a manager to remove it from the swarm.
However, this does not reconfigure the swarm to ensure that there are enough
managers to maintain a quorum in the swarm. The safe way to remove a manager
from a swarm is to demote it to a worker and then direct it to leave the quorum
without using `--force`. Only use `--force` in situations where the swarm will
no longer be used after the manager leaves, such as in a single-node swarm.

## Examples

Consider the following swarm, as seen from the manager:

```bash
$ docker node ls
ID                           HOSTNAME  STATUS  AVAILABILITY  MANAGER STATUS
7ln70fl22uw2dvjn2ft53m3q5    worker2   Ready   Active
dkp8vy1dq1kxleu9g4u78tlag    worker1   Ready   Active
dvfxp4zseq4s0rih1selh0d20 *  manager1  Ready   Active        Leader
```

To remove `worker2`, issue the following command from `worker2` itself:

```bash
$ docker swarm leave
Node left the default swarm.
```

The node will still appear in the node list, and marked as `down`. It no longer
affects swarm operation, but a long list of `down` nodes can clutter the node
list. To remove an inactive node from the list, use the [`node rm`](node_rm.md)
command.

## Related commands

* [swarm ca](swarm_ca.md)
* [node rm](node_rm.md)
* [swarm init](swarm_init.md)
* [swarm join](swarm_join.md)
* [swarm join-token](swarm_join_token.md)
* [swarm unlock](swarm_unlock.md)
* [swarm unlock-key](swarm_unlock_key.md)
* [swarm update](swarm_update.md)

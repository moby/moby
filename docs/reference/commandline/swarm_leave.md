---
title: "swarm leave"
description: "The swarm leave command description and usage"
keywords: ["swarm, leave"]
---

# swarm leave

```markdown
Usage:  docker swarm leave [OPTIONS]

Leave the swarm (workers only).

Options:
      --force   Force this node to leave the swarm, ignoring warnings
      --help    Print usage
```

When you run this command on a worker, that worker leaves the swarm.

You can use the `--force` option to on a manager to remove it from the swarm.
However, this does not reconfigure the swarm to ensure that there are enough
managers to maintain a quorum in the swarm. The safe way to remove a manager
from a swarm is to demote it to a worker and then direct it to leave the quorum
without using `--force`. Only use `--force` in situations where the swarm will
no longer be used after the manager leaves, such as in a single-node swarm.

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
To remove an inactive node, use the [`node rm`](node_rm.md) command instead.

## Related information

* [node rm](node_rm.md)
* [swarm init](swarm_init.md)
* [swarm join](swarm_join.md)
* [swarm update](swarm_update.md)

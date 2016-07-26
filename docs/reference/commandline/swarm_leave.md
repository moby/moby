<!--[metadata]>
+++
title = "swarm leave"
description = "The swarm leave command description and usage"
keywords = ["swarm, leave"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

**Warning:** this command is part of the Swarm management feature introduced in Docker 1.12, and might be subject to non backward-compatible changes.

# swarm leave

```markdown
Usage:  docker swarm leave [OPTIONS]

Leave a swarm

Options:
      --force   Force leave ignoring warnings.
      --help    Print usage
```

This command causes the node to leave the swarm.

On a manager node:
```bash
$ docker node ls
ID                           HOSTNAME  STATUS  AVAILABILITY  MANAGER STATUS
7ln70fl22uw2dvjn2ft53m3q5    worker2   Ready   Active
dkp8vy1dq1kxleu9g4u78tlag    worker1   Ready   Active        Reachable
dvfxp4zseq4s0rih1selh0d20 *  manager1  Ready   Active        Leader
```

On a worker node:
```bash
$ docker swarm leave
Node left the default swarm.
```

On a manager node:
```bash
$ docker node ls
ID                           HOSTNAME  STATUS  AVAILABILITY  MANAGER STATUS
7ln70fl22uw2dvjn2ft53m3q5    worker2   Down    Active
dkp8vy1dq1kxleu9g4u78tlag    worker1   Ready   Active        Reachable
dvfxp4zseq4s0rih1selh0d20 *  manager1  Ready   Active        Leader
```

## Related information

* [swarm init](swarm_init.md)
* [swarm join](swarm_join.md)
* [swarm update](swarm_update.md)

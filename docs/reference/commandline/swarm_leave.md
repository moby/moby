<!--[metadata]>
+++
title = "swarm leave"
description = "The swarm leave command description and usage"
keywords = ["swarm, leave"]
advisory = "rc"
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# swarm leave

```markdown
Usage:  docker swarm leave [OPTIONS]

Leave a Swarm

Options:
      --force   Force leave ignoring warnings.
      --help    Print usage
```

This command causes the node to leave the swarm.

On a manager node:
```bash
$ docker node ls
ID                           HOSTNAME  MEMBERSHIP  STATUS  AVAILABILITY  MANAGER STATUS         LEADER
7ln70fl22uw2dvjn2ft53m3q5    worker2   Accepted    Ready   Active
dkp8vy1dq1kxleu9g4u78tlag    worker1   Accepted    Ready   Active        Reachable
dvfxp4zseq4s0rih1selh0d20 *  manager1  Accepted    Ready   Active        Reachable              Yes
```

On a worker node:
```bash
$ docker swarm leave
Node left the default swarm.
```

On a manager node:
```bash
$ docker node ls
ID                           HOSTNAME  MEMBERSHIP  STATUS  AVAILABILITY  MANAGER STATUS         LEADER
7ln70fl22uw2dvjn2ft53m3q5    worker2   Accepted    Down    Active
dkp8vy1dq1kxleu9g4u78tlag    worker1   Accepted    Ready   Active        Reachable
dvfxp4zseq4s0rih1selh0d20 *  manager1  Accepted    Ready   Active        Reachable              Yes
```

## Related information

* [swarm init](swarm_init.md)
* [swarm join](swarm_join.md)
* [swarm update](swarm_update.md)

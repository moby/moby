---
redirect_from:
  - /reference/commandline/swarm_update/
description: The swarm update command description and usage
keywords:
- swarm, update
title: docker swarm update
---

**Warning:** this command is part of the Swarm management feature introduced in Docker 1.12, and might be subject to non backward-compatible changes.

```markdown
Usage:  docker swarm update [OPTIONS]

Update the swarm

Options:
      --cert-expiry duration            Validity period for node certificates (default 2160h0m0s)
      --dispatcher-heartbeat duration   Dispatcher heartbeat period (default 5s)
      --external-ca value               Specifications of one or more certificate signing endpoints
      --help                            Print usage
      --task-history-limit int          Task history retention limit (default 5)
```

Updates a swarm with new parameter values. This command must target a manager node.


```bash
$ docker swarm update --cert-expiry 720h
```

## Related information

* [swarm init](swarm_init.md)
* [swarm join](swarm_join.md)
* [swarm leave](swarm_leave.md)

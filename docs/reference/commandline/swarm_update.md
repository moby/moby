---
title: "swarm update"
description: "The swarm update command description and usage"
keywords: "swarm, update"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# swarm update

```markdown
Usage:  docker swarm update [OPTIONS]

Update the swarm

Options:
      --autolock                        Change manager autolocking setting (true|false)
      --cert-expiry duration            Validity period for node certificates (ns|us|ms|s|m|h) (default 2160h0m0s)
      --dispatcher-heartbeat duration   Dispatcher heartbeat period (ns|us|ms|s|m|h) (default 5s)
      --external-ca external-ca         Specifications of one or more certificate signing endpoints
      --help                            Print usage
      --max-snapshots uint              Number of additional Raft snapshots to retain
      --snapshot-interval uint          Number of log entries between Raft snapshots (default 10000)
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

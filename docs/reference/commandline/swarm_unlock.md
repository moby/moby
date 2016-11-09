---
title: "swarm unlock"
description: "The swarm unlock command description and usage"
keywords: "swarm, unlock"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# swarm unlock

```markdown
Usage:	docker swarm unlock

Unlock swarm

Options:
      --help   Print usage
```

Unlocks a locked manager using a user-supplied unlock key. This command must be
used to reactivate a manager after its Docker daemon restarts if the autolock
setting is turned on. The unlock key is printed at the time when autolock is
enabled, and is also available from the `docker swarm unlock-key` command.


```bash
$ docker swarm unlock
Please enter unlock key:
```

## Related information

* [swarm init](swarm_init.md)
* [swarm update](swarm_update.md)

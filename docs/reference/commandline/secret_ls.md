---
title: "secret ls"
description: "The secret ls command description and usage"
keywords: ["secret, ls"]
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# secret ls

```Markdown
Usage:	docker secret ls [OPTIONS]

List secrets

Aliases:
  ls, list

Options:
  -q, --quiet          Only display IDs
```

This command when run targeting a manager, lists secrets in the
swarm.

On a manager node:

```bash
$ docker secret ls
ID                          NAME                CREATED                                   UPDATED                                   SIZE
mhv17xfe3gh6xc4rij5orpfds   ssh-dev             2016-10-27 23:25:43.909181089 +0000 UTC   2016-10-27 23:25:43.909181089 +0000 UTC   1679
```
## Related information

* [secret create](secret_create.md)
* [secret inspect](secret_inspect.md)
* [secret rm](secret_rm.md)

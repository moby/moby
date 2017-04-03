---
title: "service rm"
description: "The service rm command description and usage"
keywords: "service, rm"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# service rm

```Markdown
Usage:	docker service rm SERVICE [SERVICE...]

Remove one or more services

Aliases:
  rm, remove

Options:
      --help   Print usage
```

## Description

Removes the specified services from the swarm. This command has to be run
targeting a manager node.

## Examples

Remove the `redis` service:

```bash
$ docker service rm redis

redis

$ docker service ls

ID  NAME  MODE  REPLICAS  IMAGE
```

> **Warning**: Unlike `docker rm`, this command does not ask for confirmation
> before removing a running service.

## Related commands

* [service create](service_create.md)
* [service inspect](service_inspect.md)
* [service logs](service_logs.md)
* [service ls](service_ls.md)
* [service scale](service_scale.md)
* [service ps](service_ps.md)
* [service update](service_update.md)

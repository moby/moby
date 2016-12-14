---
title: "service scale"
description: "The service scale command description and usage"
keywords: "service, scale"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# service scale

```markdown
Usage:  docker service scale SERVICE=REPLICAS [SERVICE=REPLICAS...]

Scale one or multiple replicated services

Options:
      --help   Print usage
```

## Examples

### Scale a service

The scale command enables you to scale one or more replicated services either up
or down to the desired number of replicas. This command cannot be applied on
services which are global mode. The command will return immediately, but the
actual scaling of the service may take some time. To stop all replicas of a
service while keeping the service active in the swarm you can set the scale to 0.

For example, the following command scales the "frontend" service to 50 tasks.

```bash
$ docker service scale frontend=50
frontend scaled to 50
```

The following command tries to scale a global service to 10 tasks and returns an error.

```
$ docker service create --mode global --name backend backend:latest
b4g08uwuairexjub6ome6usqh
$ docker service scale backend=10
backend: scale can only be used with replicated mode
```

Directly afterwards, run `docker service ls`, to see the actual number of
replicas.

```bash
$ docker service ls --filter name=frontend

ID            NAME      MODE        REPLICAS  IMAGE
3pr5mlvu3fh9  frontend  replicated  15/50     nginx:alpine
```

You can also scale a service using the [`docker service update`](service_update.md)
command. The following commands are equivalent:

```bash
$ docker service scale frontend=50
$ docker service update --replicas=50 frontend
```

### Scale multiple services

The `docker service scale` command allows you to set the desired number of
tasks for multiple services at once. The following example scales both the
backend and frontend services:

```bash
$ docker service scale backend=3 frontend=5
backend scaled to 3
frontend scaled to 5

$ docker service ls
ID            NAME      MODE        REPLICAS  IMAGE
3pr5mlvu3fh9  frontend  replicated  5/5       nginx:alpine
74nzcxxjv6fq  backend   replicated  3/3       redis:3.0.6
```

## Related information

* [service create](service_create.md)
* [service inspect](service_inspect.md)
* [service logs](service_logs.md)
* [service ls](service_ls.md)
* [service rm](service_rm.md)
* [service ps](service_ps.md)
* [service update](service_update.md)

---
redirect_from:
  - /reference/commandline/service_scale/
description: The service scale command description and usage
keywords:
- service, scale
title: docker service scale
---

**Warning:** this command is part of the Swarm management feature introduced in Docker 1.12, and might be subject to non backward-compatible changes.

```markdown
Usage:  docker service scale SERVICE=REPLICAS [SERVICE=REPLICAS...]

Scale one or multiple services

Options:
      --help   Print usage
```

## Examples

### Scale a service

The scale command enables you to scale one or more services either up or down to the desired number of replicas. The command will return immediatly, but the actual scaling of the service may take some time. To stop all replicas of a service while keeping the service active in the swarm you can set the scale to 0.


For example, the following command scales the "frontend" service to 50 tasks.

```bash
$ docker service scale frontend=50
frontend scaled to 50
```

Directly afterwards, run `docker service ls`, to see the actual number of
replicas

```bash
$ docker service ls --filter name=frontend

ID            NAME      REPLICAS  IMAGE         COMMAND
3pr5mlvu3fh9  frontend  15/50     nginx:alpine
```

You can also scale a service using the [`docker service update`](service_update.md)
command. The following commands are therefore equivalent:

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
ID            NAME      REPLICAS  IMAGE         COMMAND
3pr5mlvu3fh9  frontend  5/5       nginx:alpine
74nzcxxjv6fq  backend   3/3       redis:3.0.6
```

## Related information

* [service create](service_create.md)
* [service inspect](service_inspect.md)
* [service ls](service_ls.md)
* [service rm](service_rm.md)
* [service ps](service_ps.md)
* [service update](service_update.md)

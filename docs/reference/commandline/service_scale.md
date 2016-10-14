---
title: "service scale"
description: "The service scale command description and usage"
keywords: ["service, scale"]
---

# service scale

```markdown
Usage:  docker service scale SERVICE=REPLICAS [SERVICE=REPLICAS...]

Scale one or multiple services

Options:
      --help   Print usage
```

## Examples

### Scale a service

If you scale a service, you set the *desired* number of replicas. Even though
the command returns directly, actual scaling of the service may take some time.

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

---
title: "service ls"
description: "The service ls command description and usage"
keywords: ["service, ls"]
---

# service ls

```Markdown
Usage:	docker service ls [OPTIONS]

List services

Aliases:
  ls, list

Options:
  -f, --filter value   Filter output based on conditions provided
      --help           Print usage
  -q, --quiet          Only display IDs
```

This command when run targeting a manager, lists services are running in the
swarm.

On a manager node:
```bash
ID            NAME      REPLICAS  IMAGE         COMMAND
c8wgl7q4ndfd  frontend  5/5       nginx:alpine
dmu1ept4cxcf  redis     3/3       redis:3.0.6
```

The `REPLICAS` column shows both the *actual* and *desired* number of tasks for
the service.


## Filtering

The filtering flag (`-f` or `--filter`) format is of "key=value". If there is more
than one filter, then pass multiple flags (e.g., `--filter "foo=bar" --filter "bif=baz"`)

The currently supported filters are:

* [id](#id)
* [label](#label)
* [name](#name)

#### ID

The `id` filter matches all or part of a service's id.

```bash
$ docker service ls -f "id=0bcjw"
ID            NAME   REPLICAS  IMAGE        COMMAND
0bcjwfh8ychr  redis  1/1       redis:3.0.6
```

#### Label

The `label` filter matches services based on the presence of a `label` alone or
a `label` and a value.

The following filter matches all services with a `project` label regardless of
its value:

```bash
$ docker service ls --filter label=project
ID            NAME       REPLICAS  IMAGE         COMMAND
01sl1rp6nj5u  frontend2  1/1       nginx:alpine
36xvvwwauej0  frontend   5/5       nginx:alpine
74nzcxxjv6fq  backend    3/3       redis:3.0.6
```

The following filter matches only services with the `project` label with the
`project-a` value.

```bash
$ docker service ls --filter label=project=project-a
ID            NAME      REPLICAS  IMAGE         COMMAND
36xvvwwauej0  frontend  5/5       nginx:alpine
74nzcxxjv6fq  backend   3/3       redis:3.0.6
```


#### Name

The `name` filter matches on all or part of a tasks's name.

The following filter matches services with a name containing `redis`.

```bash
$ docker service ls --filter name=redis
ID            NAME   REPLICAS  IMAGE        COMMAND
0bcjwfh8ychr  redis  1/1       redis:3.0.6
```

## Related information

* [service create](service_create.md)
* [service inspect](service_inspect.md)
* [service rm](service_rm.md)
* [service scale](service_scale.md)
* [service ps](service_ps.md)
* [service update](service_update.md)

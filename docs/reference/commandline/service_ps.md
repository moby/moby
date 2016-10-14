---
title: "service ps"
description: "The service ps command description and usage"
keywords: ["service, tasks", "ps"]
aliases: ["/engine/reference/commandline/service_tasks/"]
---

# service ps

```Markdown
Usage:	docker service ps [OPTIONS] SERVICE

List the tasks of a service

Options:
  -a, --all            Display all tasks
  -f, --filter value   Filter output based on conditions provided
      --help           Print usage
      --no-resolve     Do not map IDs to Names
      --no-trunc       Do not truncate output
```

Lists the tasks that are running as part of the specified service. This command
has to be run targeting a manager node.


## Examples

### Listing the tasks that are part of a service

The following command shows all the tasks that are part of the `redis` service:

```bash
$ docker service ps redis
NAME                                IMAGE        NODE      DESIRED STATE  CURRENT STATE
redis.1.0qihejybwf1x5vqi8lgzlgnpq   redis:3.0.6  manager1  Running        Running 8 seconds
redis.2.bk658fpbex0d57cqcwoe3jthu   redis:3.0.6  worker2   Running        Running 9 seconds
redis.3.5ls5s5fldaqg37s9pwayjecrf   redis:3.0.6  worker1   Running        Running 9 seconds
redis.4.8ryt076polmclyihzx67zsssj   redis:3.0.6  worker1   Running        Running 9 seconds
redis.5.1x0v8yomsncd6sbvfn0ph6ogc   redis:3.0.6  manager1  Running        Running 8 seconds
redis.6.71v7je3el7rrw0osfywzs0lko   redis:3.0.6  worker2   Running        Running 9 seconds
redis.7.4l3zm9b7tfr7cedaik8roxq6r   redis:3.0.6  worker2   Running        Running 9 seconds
redis.8.9tfpyixiy2i74ad9uqmzp1q6o   redis:3.0.6  worker1   Running        Running 9 seconds
redis.9.3w1wu13yuplna8ri3fx47iwad   redis:3.0.6  manager1  Running        Running 8 seconds
redis.10.8eaxrb2fqpbnv9x30vr06i6vt  redis:3.0.6  manager1  Running        Running 8 seconds
```


## Filtering

The filtering flag (`-f` or `--filter`) format is a `key=value` pair. If there
is more than one filter, then pass multiple flags (e.g. `--filter "foo=bar" --filter "bif=baz"`).
Multiple filter flags are combined as an `OR` filter. For example,
`-f name=redis.1 -f name=redis.7` returns both `redis.1` and `redis.7` tasks.

The currently supported filters are:

* [id](#id)
* [name](#name)
* [node](#node)
* [desired-state](#desired-state)


#### ID

The `id` filter matches on all or a prefix of a task's ID.

```bash
$ docker service ps -f "id=8" redis
NAME                                IMAGE        NODE      DESIRED STATE  CURRENT STATE
redis.4.8ryt076polmclyihzx67zsssj   redis:3.0.6  worker1   Running        Running 9 seconds
redis.10.8eaxrb2fqpbnv9x30vr06i6vt  redis:3.0.6  manager1  Running        Running 8 seconds
```

#### Name

The `name` filter matches on task names.

```bash
$ docker service ps -f "name=redis.1" redis
NAME                                IMAGE        NODE      DESIRED STATE  CURRENT STATE
redis.1.0qihejybwf1x5vqi8lgzlgnpq   redis:3.0.6  manager1  Running        Running 8 seconds
```


#### Node

The `node` filter matches on a node name or a node ID.

```bash
$ docker service ps -f "node=manager1" redis
NAME                                IMAGE        NODE      DESIRED STATE  CURRENT STATE
redis.1.0qihejybwf1x5vqi8lgzlgnpq   redis:3.0.6  manager1  Running        Running 8 seconds
redis.5.1x0v8yomsncd6sbvfn0ph6ogc   redis:3.0.6  manager1  Running        Running 8 seconds
redis.9.3w1wu13yuplna8ri3fx47iwad   redis:3.0.6  manager1  Running        Running 8 seconds
redis.10.8eaxrb2fqpbnv9x30vr06i6vt  redis:3.0.6  manager1  Running        Running 8 seconds
```


#### desired-state

The `desired-state` filter can take the values `running`, `shutdown`, and `accepted`.


## Related information

* [service create](service_create.md)
* [service inspect](service_inspect.md)
* [service ls](service_ls.md)
* [service rm](service_rm.md)
* [service scale](service_scale.md)
* [service update](service_update.md)

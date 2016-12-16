---
title: "service ps"
description: "The service ps command description and usage"
keywords: "service, tasks, ps"
aliases: ["/engine/reference/commandline/service_tasks/"]
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# service ps

```Markdown
Usage:  docker service ps [OPTIONS] SERVICE

List the tasks of a service

Options:
  -f, --filter filter   Filter output based on conditions provided
      --help            Print usage
      --no-resolve      Do not map IDs to Names
      --no-trunc        Do not truncate output
  -q, --quiet           Only display task IDs
```

Lists the tasks that are running as part of the specified service. This command
has to be run targeting a manager node.

## Examples

### Listing the tasks that are part of a service

The following command shows all the tasks that are part of the `redis` service:

```bash
$ docker service ps redis

ID             NAME      IMAGE        NODE      DESIRED STATE  CURRENT STATE          ERROR  PORTS
0qihejybwf1x   redis.1   redis:3.0.5  manager1  Running        Running 8 seconds
bk658fpbex0d   redis.2   redis:3.0.5  worker2   Running        Running 9 seconds
5ls5s5fldaqg   redis.3   redis:3.0.5  worker1   Running        Running 9 seconds
8ryt076polmc   redis.4   redis:3.0.5  worker1   Running        Running 9 seconds
1x0v8yomsncd   redis.5   redis:3.0.5  manager1  Running        Running 8 seconds
71v7je3el7rr   redis.6   redis:3.0.5  worker2   Running        Running 9 seconds
4l3zm9b7tfr7   redis.7   redis:3.0.5  worker2   Running        Running 9 seconds
9tfpyixiy2i7   redis.8   redis:3.0.5  worker1   Running        Running 9 seconds
3w1wu13yupln   redis.9   redis:3.0.5  manager1  Running        Running 8 seconds
8eaxrb2fqpbn   redis.10  redis:3.0.5  manager1  Running        Running 8 seconds
```

In addition to _running_ tasks, the output also shows the task history. For
example, after updating the service to use the `redis:3.0.6` image, the output
may look like this:

```bash
$ docker service ps redis

ID            NAME         IMAGE        NODE      DESIRED STATE  CURRENT STATE                   ERROR  PORTS
50qe8lfnxaxk  redis.1      redis:3.0.6  manager1  Running        Running 6 seconds ago
ky2re9oz86r9   \_ redis.1  redis:3.0.5  manager1  Shutdown       Shutdown 8 seconds ago
3s46te2nzl4i  redis.2      redis:3.0.6  worker2   Running        Running less than a second ago
nvjljf7rmor4   \_ redis.2  redis:3.0.6  worker2   Shutdown       Rejected 23 seconds ago        "No such image: redis@sha256:6…"
vtiuz2fpc0yb   \_ redis.2  redis:3.0.5  worker2   Shutdown       Shutdown 1 second ago
jnarweeha8x4  redis.3      redis:3.0.6  worker1   Running        Running 3 seconds ago
vs448yca2nz4   \_ redis.3  redis:3.0.5  worker1   Shutdown       Shutdown 4 seconds ago
jf1i992619ir  redis.4      redis:3.0.6  worker1   Running        Running 10 seconds ago
blkttv7zs8ee   \_ redis.4  redis:3.0.5  worker1   Shutdown       Shutdown 11 seconds ago
```

The number of items in the task history is determined by the
`--task-history-limit` option that was set when initializing the swarm. You can
change the task history retention limit using the
[`docker swarm update`](swarm_update.md) command.

When deploying a service, docker resolves the digest for the service's
image, and pins the service to that digest. The digest is not shown by
default, but is printed if `--no-trunc` is used. The `--no-trunc` option
also shows the non-truncated task ID, and error-messages, as can be seen below;

```bash
$ docker service ps --no-trunc redis

ID                          NAME         IMAGE                                                                                NODE      DESIRED STATE  CURRENT STATE            ERROR                                                                                           PORTS
50qe8lfnxaxksi9w2a704wkp7   redis.1      redis:3.0.6@sha256:6a692a76c2081888b589e26e6ec835743119fe453d67ecf03df7de5b73d69842  manager1  Running        Running 5 minutes ago
ky2re9oz86r9556i2szb8a8af   \_ redis.1   redis:3.0.5@sha256:f8829e00d95672c48c60f468329d6693c4bdd28d1f057e755f8ba8b40008682e  worker2   Shutdown       Shutdown 5 minutes ago
bk658fpbex0d57cqcwoe3jthu   redis.2      redis:3.0.6@sha256:6a692a76c2081888b589e26e6ec835743119fe453d67ecf03df7de5b73d69842  worker2   Running        Running 5 seconds
nvjljf7rmor4htv7l8rwcx7i7   \_ redis.2   redis:3.0.6@sha256:6a692a76c2081888b589e26e6ec835743119fe453d67ecf03df7de5b73d69842  worker2   Shutdown       Rejected 5 minutes ago   "No such image: redis@sha256:6a692a76c2081888b589e26e6ec835743119fe453d67ecf03df7de5b73d69842"
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

ID             NAME      IMAGE        NODE      DESIRED STATE  CURRENT STATE      ERROR  PORTS
8ryt076polmc   redis.4   redis:3.0.6  worker1   Running        Running 9 seconds
8eaxrb2fqpbn   redis.10  redis:3.0.6  manager1  Running        Running 8 seconds
```

#### Name

The `name` filter matches on task names.

```bash
$ docker service ps -f "name=redis.1" redis
ID            NAME     IMAGE        NODE      DESIRED STATE  CURRENT STATE      ERROR  PORTS
qihejybwf1x5  redis.1  redis:3.0.6  manager1  Running        Running 8 seconds
```


#### Node

The `node` filter matches on a node name or a node ID.

```bash
$ docker service ps -f "node=manager1" redis
ID            NAME      IMAGE        NODE      DESIRED STATE  CURRENT STATE      ERROR  PORTS
0qihejybwf1x  redis.1   redis:3.0.6  manager1  Running        Running 8 seconds
1x0v8yomsncd  redis.5   redis:3.0.6  manager1  Running        Running 8 seconds
3w1wu13yupln  redis.9   redis:3.0.6  manager1  Running        Running 8 seconds
8eaxrb2fqpbn  redis.10  redis:3.0.6  manager1  Running        Running 8 seconds
```


#### desired-state

The `desired-state` filter can take the values `running`, `shutdown`, and `accepted`.


## Related information

* [service create](service_create.md)
* [service inspect](service_inspect.md)
* [service logs](service_logs.md)
* [service ls](service_ls.md)
* [service rm](service_rm.md)
* [service scale](service_scale.md)
* [service update](service_update.md)

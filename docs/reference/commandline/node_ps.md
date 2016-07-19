<!--[metadata]>
+++
title = "node ps"
description = "The node ps command description and usage"
keywords = ["node, tasks", "ps"]
aliases = ["/engine/reference/commandline/node_tasks/"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

**Warning:** this command is part of the Swarm management feature introduced in Docker 1.12, and might be subject to non backward-compatible changes.

# node ps

```markdown
Usage:  docker node ps [OPTIONS] self|NODE

List tasks running on a node

Options:
  -a, --all            Display all instances
  -f, --filter value   Filter output based on conditions provided
      --help           Print usage
      --no-resolve     Do not map IDs to Names
```

Lists all the tasks on a Node that Docker knows about. You can filter using the `-f` or `--filter` flag. Refer to the [filtering](#filtering) section for more information about available filter options.

Example output:

    $ docker node ps swarm-manager1
    ID                         NAME      SERVICE  IMAGE        LAST STATE          DESIRED STATE  NODE
    7q92v0nr1hcgts2amcjyqg3pq  redis.1   redis    redis:3.0.6  Running 5 hours     Running        swarm-manager1
    b465edgho06e318egmgjbqo4o  redis.6   redis    redis:3.0.6  Running 29 seconds  Running        swarm-manager1
    bg8c07zzg87di2mufeq51a2qp  redis.7   redis    redis:3.0.6  Running 5 seconds   Running        swarm-manager1
    dkkual96p4bb3s6b10r7coxxt  redis.9   redis    redis:3.0.6  Running 5 seconds   Running        swarm-manager1
    0tgctg8h8cech4w0k0gwrmr23  redis.10  redis    redis:3.0.6  Running 5 seconds   Running        swarm-manager1


## Filtering

The filtering flag (`-f` or `--filter`) format is of "key=value". If there is more
than one filter, then pass multiple flags (e.g., `--filter "foo=bar" --filter "bif=baz"`)

The currently supported filters are:

* [name](#name)
* [id](#id)
* [label](#label)
* [desired-state](#desired-state)

#### name

The `name` filter matches on all or part of a task's name.

The following filter matches all tasks with a name containing the `redis` string.

    $ docker node ps -f name=redis swarm-manager1
    ID                         NAME      SERVICE  IMAGE        LAST STATE          DESIRED STATE  NODE
    7q92v0nr1hcgts2amcjyqg3pq  redis.1   redis    redis:3.0.6  Running 5 hours     Running        swarm-manager1
    b465edgho06e318egmgjbqo4o  redis.6   redis    redis:3.0.6  Running 29 seconds  Running        swarm-manager1
    bg8c07zzg87di2mufeq51a2qp  redis.7   redis    redis:3.0.6  Running 5 seconds   Running        swarm-manager1
    dkkual96p4bb3s6b10r7coxxt  redis.9   redis    redis:3.0.6  Running 5 seconds   Running        swarm-manager1
    0tgctg8h8cech4w0k0gwrmr23  redis.10  redis    redis:3.0.6  Running 5 seconds   Running        swarm-manager1


#### id

The `id` filter matches a task's id.

    $ docker node ps -f id=bg8c07zzg87di2mufeq51a2qp swarm-manager1
    ID                         NAME      SERVICE  IMAGE        LAST STATE             DESIRED STATE  NODE
    bg8c07zzg87di2mufeq51a2qp  redis.7   redis    redis:3.0.6  Running 5 seconds      Running        swarm-manager1


#### label

The `label` filter matches tasks based on the presence of a `label` alone or a `label` and a
value.

The following filter matches tasks with the `usage` label regardless of its value.

```bash
$ docker node ps -f "label=usage"
ID                         NAME     SERVICE  IMAGE        LAST STATE          DESIRED STATE  NODE
b465edgho06e318egmgjbqo4o  redis.6  redis    redis:3.0.6  Running 10 minutes  Running        swarm-manager1
bg8c07zzg87di2mufeq51a2qp  redis.7  redis    redis:3.0.6  Running 9 minutes   Running        swarm-manager1
```


#### desired-state

The `desired-state` filter can take the values `running` and `accepted`.


## Related information

* [node inspect](node_inspect.md)
* [node update](node_update.md)
* [node ls](node_ls.md)
* [node rm](node_rm.md)

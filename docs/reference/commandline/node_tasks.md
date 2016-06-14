<!--[metadata]>
+++
title = "node tasks"
description = "The node tasks command description and usage"
keywords = ["node, tasks"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

**Warning:** this command is part of the Swarm management feature introduced in Docker 1.12, and might be subject to non backward-compatible changes.

# node tasks

    Usage:  docker node tasks [OPTIONS] NODE

    List tasks running on a node

    Options:
      -a, --all            Display all instances
      -f, --filter value   Filter output based on conditions provided
      --help           Print usage
      -n, --no-resolve     Do not map IDs to Names

Lists all the tasks on a Node that Docker knows about. You can filter using the `-f` or `--filter` flag. Refer to the [filtering](#filtering) section for more information about available filter options.

Example output:

    $ docker node tasks swarm-master
    ID                         NAME     SERVICE  IMAGE        DESIRED STATE  LAST STATE       NODE
    dx2g0fe3zsdb6y6q453f8dqw2  redis.1  redis    redis:3.0.6  RUNNING        RUNNING 2 hours  swarm-master
    f33pcf8lwhs4c1t4kq8szwzta  redis.4  redis    redis:3.0.6  RUNNING        RUNNING 2 hours  swarm-master
    5v26yzixl3one3ptjyqqbd0ro  redis.5  redis    redis:3.0.6  RUNNING        RUNNING 2 hours  swarm-master
    adcaphlhsfr30d47lby6walg6  redis.8  redis    redis:3.0.6  RUNNING        RUNNING 2 hours  swarm-master
    chancjvk9tex6768uzzacslq2  redis.9  redis    redis:3.0.6  RUNNING        RUNNING 2 hours  swarm-master


## Filtering

The filtering flag (`-f` or `--filter`) format is of "key=value". If there is more
than one filter, then pass multiple flags (e.g., `--filter "foo=bar" --filter "bif=baz"`)

The currently supported filters are:

* name
* id
* label
* desired_state

### name

The `name` filter matches on all or part of a task's name.

The following filter matches all tasks with a name containing the `redis` string.

    $ docker node tasks -f name=redis swarm-master
    ID                         NAME     SERVICE  IMAGE        DESIRED STATE  LAST STATE       NODE
    dx2g0fe3zsdb6y6q453f8dqw2  redis.1  redis    redis:3.0.6  RUNNING        RUNNING 2 hours  swarm-master
    f33pcf8lwhs4c1t4kq8szwzta  redis.4  redis    redis:3.0.6  RUNNING        RUNNING 2 hours  swarm-master
    5v26yzixl3one3ptjyqqbd0ro  redis.5  redis    redis:3.0.6  RUNNING        RUNNING 2 hours  swarm-master
    adcaphlhsfr30d47lby6walg6  redis.8  redis    redis:3.0.6  RUNNING        RUNNING 2 hours  swarm-master
    chancjvk9tex6768uzzacslq2  redis.9  redis    redis:3.0.6  RUNNING        RUNNING 2 hours  swarm-master


### id

The `id` filter matches a task's id.

    $ docker node tasks -f id=f33pcf8lwhs4c1t4kq8szwzta swarm-master
    ID                         NAME     SERVICE  IMAGE        DESIRED STATE  LAST STATE       NODE
    f33pcf8lwhs4c1t4kq8szwzta  redis.4  redis    redis:3.0.6  RUNNING        RUNNING 2 hours  swarm-master


#### label

The `label` filter matches tasks based on the presence of a `label` alone or a `label` and a
value.

The following filter matches tasks with the `usage` label regardless of its value.

```bash
$ docker node tasks -f "label=usage"
ID                         NAME     SERVICE  IMAGE        DESIRED STATE  LAST STATE       NODE
dx2g0fe3zsdb6y6q453f8dqw2  redis.1  redis    redis:3.0.6  RUNNING        RUNNING 2 hours  swarm-master
f33pcf8lwhs4c1t4kq8szwzta  redis.4  redis    redis:3.0.6  RUNNING        RUNNING 2 hours  swarm-master
```


## Related information

* [node inspect](node_inspect.md)
* [node update](node_update.md)
* [node ls](node_ls.md)
* [node rm](node_rm.md)

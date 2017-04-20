---
title: "stack ps"
description: "The stack ps command description and usage"
keywords: "stack, ps"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# stack ps

```markdown
Usage:  docker stack ps [OPTIONS] STACK

List the tasks in the stack

Options:
  -f, --filter filter   Filter output based on conditions provided
      --help            Print usage
      --no-resolve      Do not map IDs to Names
      --no-trunc        Do not truncate output
```

## Description

Lists the tasks that are running as part of the specified stack. This
command has to be run targeting a manager node.

## Examples

```bash
$ docker stack ps myapp

ID            NAME         IMAGE           NODE           DESIRED STATE  CURRENT STATE            ERROR  PORTS
k6b0qp7gkv8z  myapp_db.1   mariadb:latest  node1          Running        Preparing 3 seconds ago         
vqw9o7xlh7xo  myapp_web.1  nginx:latest    node1          Running        Starting 4 seconds ago          
fi7zmadwfyjp  myapp_db.2   mariadb:latest  node2          Running        Preparing 3 seconds ago         
wycqmdxwact6  myapp_web.2  nginx:latest    node2          Running        Running 2 seconds ago           
4zblbvdm6ge7  myapp_web.3  nginx:latest    node2          Running        Starting 2 seconds ago          
```

### Filtering

The filtering flag (`-f` or `--filter`) format is a `key=value` pair. If there
is more than one filter, then pass multiple flags (e.g. `--filter "foo=bar" --filter "bif=baz"`).
Multiple filter flags are combined as an `OR` filter. For example,
`-f name=redis.1 -f name=redis.7` returns both `redis.1` and `redis.7` tasks.

The currently supported filters are:

* [id](#id)
* [name](#name)
* [node](#node)
* [desired-state](#desired-state)

#### id

The `id` filter matches on all or a prefix of a task's ID.

```bash
$ docker stack ps -f "id=k6" myapp

ID            NAME        IMAGE           NODE           DESIRED STATE  CURRENT STATE          ERROR  PORTS
k6b0qp7gkv8z  myapp_db.1  mariadb:latest  node1          Running        Running 2 minutes ago         
```

#### name

The `name` filter matches on task names.

```bash
$ docker stack ps -f "name=myapp_web.2" myapp

ID            NAME         IMAGE         NODE           DESIRED STATE  CURRENT STATE          ERROR  PORTS
wycqmdxwact6  myapp_web.2  nginx:latest  node2          Running        Running 5 minutes ago         
```

#### node

The `node` filter matches on a node name or a node ID.

```bash
$ docker stack ps -f "node=node2" myapp

ID            NAME         IMAGE           NODE           DESIRED STATE  CURRENT STATE          ERROR  PORTS
fi7zmadwfyjp  myapp_db.2   mariadb:latest  node2          Running        Running 5 minutes ago         
wycqmdxwact6  myapp_web.2  nginx:latest    node2          Running        Running 5 minutes ago         
4zblbvdm6ge7  myapp_web.3  nginx:latest    node2          Running        Running 5 minutes ago         
```

#### desired-state

The `desired-state` filter can take the values `running`, `shutdown`, and `accepted`.

```bash
$ docker stack ps -f "desired-state=running" myapp

ID            NAME         IMAGE           NODE           DESIRED STATE  CURRENT STATE          ERROR  PORTS
k6b0qp7gkv8z  myapp_db.1   mariadb:latest  node1          Running        Running 6 minutes ago         
vqw9o7xlh7xo  myapp_web.1  nginx:latest    node1          Running        Running 6 minutes ago         
fi7zmadwfyjp  myapp_db.2   mariadb:latest  node2          Running        Running 6 minutes ago         
wycqmdxwact6  myapp_web.2  nginx:latest    node2          Running        Running 6 minutes ago         
4zblbvdm6ge7  myapp_web.3  nginx:latest    node2          Running        Running 6 minutes ago         
```

## Related commands

* [stack deploy](stack_deploy.md)
* [stack ls](stack_ls.md)
* [stack rm](stack_rm.md)
* [stack services](stack_services.md)

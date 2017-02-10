---
title: "node ls"
description: "The node ls command description and usage"
keywords: "node, list"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# node ls

```markdown
Usage:  docker node ls [OPTIONS]

List nodes in the swarm

Aliases:
  ls, list

Options:
  -f, --filter value   Filter output based on conditions provided
      --help           Print usage
  -q, --quiet          Only display IDs
```

## Description

Lists all the nodes that the Docker Swarm manager knows about. You can filter
using the `-f` or `--filter` flag. Refer to the [filtering](#filtering) section
for more information about available filter options.

## Examples

```bash
$ docker node ls

ID                           HOSTNAME        STATUS  AVAILABILITY  MANAGER STATUS
1bcef6utixb0l0ca7gxuivsj0    swarm-worker2   Ready   Active
38ciaotwjuritcdtn9npbnkuz    swarm-worker1   Ready   Active
e216jshn25ckzbvmwlnh5jr3g *  swarm-manager1  Ready   Active        Leader
```

### Filtering

The filtering flag (`-f` or `--filter`) format is of "key=value". If there is more
than one filter, then pass multiple flags (e.g., `--filter "foo=bar" --filter "bif=baz"`)

The currently supported filters are:

* [id](node_ls.md#id)
* [label](node_ls.md#label)
* [membership](node_ls.md#membership)
* [name](node_ls.md#name)
* [role](node_ls.md#role)

#### id

The `id` filter matches all or part of a node's id.

```bash
$ docker node ls -f id=1

ID                         HOSTNAME       STATUS  AVAILABILITY  MANAGER STATUS
1bcef6utixb0l0ca7gxuivsj0  swarm-worker2  Ready   Active
```

#### label

The `label` filter matches nodes based on engine labels and on the presence of a `label` alone or a `label` and a value. Node labels are currently not used for filtering.

The following filter matches nodes with the `foo` label regardless of its value.

```bash
$ docker node ls -f "label=foo"

ID                         HOSTNAME       STATUS  AVAILABILITY  MANAGER STATUS
1bcef6utixb0l0ca7gxuivsj0  swarm-worker2  Ready   Active
```

#### membersip

The `membership` filter matches nodes based on the presence of a `membership` and a value
`accepted` or `pending`.

The following filter matches nodes with the `membership` of `accepted`.

```bash
$ docker node ls -f "membership=accepted"

ID                           HOSTNAME        STATUS  AVAILABILITY  MANAGER STATUS
1bcef6utixb0l0ca7gxuivsj0    swarm-worker2   Ready   Active
38ciaotwjuritcdtn9npbnkuz    swarm-worker1   Ready   Active
```

#### name

The `name` filter matches on all or part of a node hostname.

The following filter matches the nodes with a name equal to `swarm-master` string.

```bash
$ docker node ls -f name=swarm-manager1

ID                           HOSTNAME        STATUS  AVAILABILITY  MANAGER STATUS
e216jshn25ckzbvmwlnh5jr3g *  swarm-manager1  Ready   Active        Leader
```

#### role

The `role` filter matches nodes based on the presence of a `role` and a value `worker` or `manager`.

The following filter matches nodes with the `manager` role.

```bash
$ docker node ls -f "role=manager"

ID                           HOSTNAME        STATUS  AVAILABILITY  MANAGER STATUS
e216jshn25ckzbvmwlnh5jr3g *  swarm-manager1  Ready   Active        Leader
```

## Related commands

* [node demote](node_demote.md)
* [node inspect](node_inspect.md)
* [node promote](node_promote.md)
* [node ps](node_ps.md)
* [node rm](node_rm.md)
* [node update](node_update.md)

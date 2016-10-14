---
title: "node ls"
description: "The node ls command description and usage"
keywords: ["node, list"]
---

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

Lists all the nodes that the Docker Swarm manager knows about. You can filter using the `-f` or `--filter` flag. Refer to the [filtering](#filtering) section for more information about available filter options.

Example output:

```bash
$ docker node ls

ID                           HOSTNAME        STATUS  AVAILABILITY  MANAGER STATUS
1bcef6utixb0l0ca7gxuivsj0    swarm-worker2   Ready   Active
38ciaotwjuritcdtn9npbnkuz    swarm-worker1   Ready   Active
e216jshn25ckzbvmwlnh5jr3g *  swarm-manager1  Ready   Active        Leader
```

## Filtering

The filtering flag (`-f` or `--filter`) format is of "key=value". If there is more
than one filter, then pass multiple flags (e.g., `--filter "foo=bar" --filter "bif=baz"`)

The currently supported filters are:

* name
* id
* label

### name

The `name` filter matches on all or part of a node name.

The following filter matches the node with a name equal to `swarm-master` string.

```bash
$ docker node ls -f name=swarm-manager1

ID                           HOSTNAME        STATUS  AVAILABILITY  MANAGER STATUS
e216jshn25ckzbvmwlnh5jr3g *  swarm-manager1  Ready   Active        Leader
```

### id

The `id` filter matches all or part of a node's id.

```bash
$ docker node ls -f id=1

ID                         HOSTNAME       STATUS  AVAILABILITY  MANAGER STATUS
1bcef6utixb0l0ca7gxuivsj0  swarm-worker2  Ready   Active
```

#### label

The `label` filter matches tasks based on the presence of a `label` alone or a `label` and a
value.

The following filter matches nodes with the `usage` label regardless of its value.

```bash
$ docker node ls -f "label=foo"

ID                         HOSTNAME       STATUS  AVAILABILITY  MANAGER STATUS
1bcef6utixb0l0ca7gxuivsj0  swarm-worker2  Ready   Active
```


## Related information

* [node inspect](node_inspect.md)
* [node update](node_update.md)
* [node ps](node_ps.md)
* [node rm](node_rm.md)

<!--[metadata]>
+++
title = "node ls"
description = "The node ls command description and usage"
keywords = ["node, list"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

**Warning:** this command is part of the Swarm management feature introduced in Docker 1.12, and might be subject to non backward-compatible changes.

# node ls

    Usage:  docker node ls [OPTIONS]

    List nodes in the swarm

    Aliases:
      ls, list

    Options:
      -f, --filter value   Filter output based on conditions provided
          --help           Print usage
      -q, --quiet          Only display IDs

Lists all the nodes that the Docker Swarm manager knows about. You can filter using the `-f` or `--filter` flag. Refer to the [filtering](#filtering) section for more information about available filter options.

Example output:

    $ docker node ls
    ID              NAME           STATUS  AVAILABILITY/MEMBERSHIP  MANAGER STATUS  LEADER
    0gac67oclbxq    swarm-master   READY   ACTIVE                   REACHABLE       Yes
    0pwvm3ve66q7    swarm-node-02  READY   ACTIVE                                   
    15xwihgw71aw *  swarm-node-01  READY   ACTIVE                   REACHABLE       


## Filtering

The filtering flag (`-f` or `--filter`) format is of "key=value". If there is more
than one filter, then pass multiple flags (e.g., `--filter "foo=bar" --filter "bif=baz"`)

The currently supported filters are:

* name
* id
* label
* desired_state

### name

The `name` filter matches on all or part of a tasks's name.

The following filter matches the node with a name equal to `swarm-master` string.

    $ docker node ls -f name=swarm-master
    ID              NAME          STATUS  AVAILABILITY/MEMBERSHIP  MANAGER STATUS  LEADER
    0gac67oclbxq *  swarm-master  READY   ACTIVE                   REACHABLE       Yes

### id

The `id` filter matches all or part of a node's id.

    $ docker node ls -f id=0
    ID              NAME           STATUS  AVAILABILITY/MEMBERSHIP  MANAGER STATUS  LEADER
    0gac67oclbxq *  swarm-master   READY   ACTIVE                   REACHABLE       Yes
    0pwvm3ve66q7    swarm-node-02  READY   ACTIVE                             


#### label

The `label` filter matches tasks based on the presence of a `label` alone or a `label` and a
value.

The following filter matches nodes with the `usage` label regardless of its value.

```bash
$ docker node ls -f "label=foo"
ID              NAME           STATUS  AVAILABILITY/MEMBERSHIP  MANAGER STATUS  LEADER
15xwihgw71aw *  swarm-node-01  READY   ACTIVE                   REACHABLE       
```


## Related information

* [node inspect](node_inspect.md)
* [node update](node_update.md)
* [node tasks](node_tasks.md)
* [node rm](node_rm.md)

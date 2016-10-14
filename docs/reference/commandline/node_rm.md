---
title: "node rm"
description: "The node rm command description and usage"
keywords: ["node, remove"]
---

# node rm

```markdown
Usage:  docker node rm [OPTIONS] NODE [NODE...]

Remove one or more nodes from the swarm

Aliases:
  rm, remove

Options:
      --force  Force remove an active node
      --help   Print usage
```

When run from a manager node, removes the specified nodes from a swarm.


Example output:

```nohighlight
$ docker node rm swarm-node-02

Node swarm-node-02 removed from swarm
```

Removes the specified nodes from the swarm, but only if the nodes are in the
down state. If you attempt to remove an active node you will receive an error:

```nohighlight
$ docker node rm swarm-node-03

Error response from daemon: rpc error: code = 9 desc = node swarm-node-03 is not
down and can't be removed
```

If you lose access to a worker node or need to shut it down because it has been
compromised or is not behaving as expected, you can use the `--force` option.
This may cause transient errors or interruptions, depending on the type of task
being run on the node.

```nohighlight
$ docker node rm --force swarm-node-03

Node swarm-node-03 removed from swarm
```

A manager node must be demoted to a worker node (using `docker node demote`)
before you can remove it from the swarm.

## Related information

* [node inspect](node_inspect.md)
* [node update](node_update.md)
* [node demote](node_demote.md)
* [node ps](node_ps.md)
* [node ls](node_ls.md)

<!--[metadata]>
+++
title = "node rm"
description = "The node rm command description and usage"
keywords = ["node, remove"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

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

Removes specified nodes from a swarm.


Example output:

    $ docker node rm swarm-node-02
    Node swarm-node-02 removed from swarm

Removes nodes from the swarm that are in the down state. Attempting to remove
an active node will result in an error:

```bash
$ docker node rm swarm-node-03
Error response from daemon: rpc error: code = 9 desc = node swarm-node-03 is not down and can't be removed
```

If a worker node becomes compromised, exhibits unexpected or unwanted behavior, or if you lose access to it so
that a clean shutdown is impossible, you can use the force option.

```bash
$ docker node rm --force swarm-node-03
Node swarm-node-03 removed from swarm
```

Note that manager nodes have to be demoted to worker nodes before they can be removed
from the cluster.

## Related information

* [node inspect](node_inspect.md)
* [node update](node_update.md)
* [node ps](node_ps.md)
* [node ls](node_ls.md)

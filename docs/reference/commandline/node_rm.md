<!--[metadata]>
+++
title = "node rm"
description = "The node rm command description and usage"
keywords = ["node, remove"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

**Warning:** this command is part of the Swarm management feature introduced in Docker 1.12, and might be subject to non backward-compatible changes.

# node rm

```markdown
Usage:  docker node rm NODE [NODE...]

Remove a node from the swarm

Aliases:
  rm, remove

Options:
      --help   Print usage
```

Removes specified nodes from a swarm.


Example output:

    $ docker node rm swarm-node-02
    Node swarm-node-02 removed from swarm


## Related information

* [node inspect](node_inspect.md)
* [node update](node_update.md)
* [node ps](node_ps.md)
* [node ls](node_ls.md)

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
Usage:  docker node rm NODE [NODE...]

Remove one or more nodes from the swarm

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

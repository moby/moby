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

	Usage:	docker node rm NODE [NODE...]

	Remove a node from the swarm

	Aliases:
	  rm, remove

	Options:
	      --help   Print usage

Removes nodes that are specified. 

Example output:

    $ docker node rm swarm-node-02
    Node swarm-node-02 removed from Swarm


## Related information

* [node inspect](node_inspect.md)
* [node update](node_update.md)
* [node tasks](node_tasks.md)
* [node ls](node_ls.md)

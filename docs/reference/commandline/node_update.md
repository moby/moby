<!--[metadata]>
+++
title = "node update"
description = "The node update command description and usage"
keywords = ["resources, update, dynamically"]
advisory = "rc"
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

## update

```markdown
Usage:  docker node update [OPTIONS] NODE

Update a node

Options:
      --availability string   Availability of the node (active/pause/drain)
      --help                  Print usage
      --membership string     Membership of the node (accepted/rejected)
      --role string           Role of the node (worker/manager)
```


## Related information

* [node inspect](node_inspect.md)
* [node tasks](node_tasks.md)
* [node ls](node_ls.md)
* [node rm](node_rm.md)

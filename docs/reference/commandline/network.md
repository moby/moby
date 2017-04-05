---
title: "network"
description: "The network command description and usage"
keywords: "network"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# network

```markdown
Usage:  docker network COMMAND

Manage networks

Options:
      --help   Print usage

Commands:
  connect     Connect a container to a network
  create      Create a network
  disconnect  Disconnect a container from a network
  inspect     Display detailed information on one or more networks
  ls          List networks
  prune       Remove all unused networks
  rm          Remove one or more networks

Run 'docker network COMMAND --help' for more information on a command.
```

## Description

Manage networks. You can use subcommands to create, inspect, list, remove,
prune, connect, and disconnect networks.

## Related commands

* [network create](network_create.md)
* [network inspect](network_inspect.md)
* [network list](network_list.md)
* [network rm](network_rm.md)
* [network prune](network_prune.md)
* [network connect](network_connect.md)
* [network disconnect](network_disconnect.md)

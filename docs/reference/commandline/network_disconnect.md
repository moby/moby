---
title: "network disconnect"
description: "The network disconnect command description and usage"
keywords: "network, disconnect, user-defined"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# network disconnect

```markdown
Usage:  docker network disconnect [OPTIONS] NETWORK CONTAINER

Disconnect a container from a network

Options:
  -f, --force   Force the container to disconnect from a network
      --help    Print usage
```

## Description

Disconnects a container from a network. The container must be running to
disconnect it from the network.

## Examples

```bash
  $ docker network disconnect multi-host-network container1
```


## Related commands

* [network inspect](network_inspect.md)
* [network connect](network_connect.md)
* [network create](network_create.md)
* [network ls](network_ls.md)
* [network rm](network_rm.md)
* [network prune](network_prune.md)
* [Understand Docker container networks](https://docs.docker.com/engine/userguide/networking/)

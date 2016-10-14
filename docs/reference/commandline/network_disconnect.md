---
title: "network disconnect"
description: "The network disconnect command description and usage"
keywords: ["network, disconnect, user-defined"]
---

# network disconnect

```markdown
Usage:  docker network disconnect [OPTIONS] NETWORK CONTAINER

Disconnect a container from a network

Options:
  -f, --force   Force the container to disconnect from a network
      --help    Print usage
```

Disconnects a container from a network. The container must be running to disconnect it from the network.

```bash
  $ docker network disconnect multi-host-network container1
```


## Related information

* [network inspect](network_inspect.md)
* [network connect](network_connect.md)
* [network create](network_create.md)
* [network ls](network_ls.md)
* [network rm](network_rm.md)
* [Understand Docker container networks](../../userguide/networking/index.md)

<!--[metadata]>
+++
title = "network rm"
description = "the network rm command description and usage"
keywords = ["network, rm, user-defined"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# network rm

    Usage:  docker network rm [OPTIONS] NAME | ID

    Deletes a network

      --help=false       Print usage

Removes a network by name or identifier. To remove a network, you must first disconnect any containers connected to it.

```bash
  $ docker network rm my-network
```

## Related information

* [network disconnect ](network_disconnect.md)
* [network connect](network_connect.md)
* [network create](network_create.md)
* [network ls](network_ls.md)
* [network inspect](network_inspect.md)
* [Understand Docker container networks](../../userguide/networking/dockernetworks.md)

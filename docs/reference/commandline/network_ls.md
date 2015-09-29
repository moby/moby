<!--[metadata]>
+++
title = "network ls"
description = "The network ls command description and usage"
keywords = ["network, list"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# docker network ls

    Usage:  docker network ls [OPTIONS]

    Lists all the networks created by the user
      --help=false          Print usage
      -l, --latest=false    Show the latest network created
      -n=-1                 Show n last created networks
      --no-trunc=false      Do not truncate the output
      -q, --quiet=false     Only display numeric IDs

Lists all the networks Docker knows about. This include the networks that spans across multiple hosts in a cluster.

Example output:

```
    $ sudo docker network ls
    NETWORK ID          NAME                DRIVER
    7fca4eb8c647        bridge              bridge
    9f904ee27bf5        none                null
    cf03ee007fb4        host                host
```

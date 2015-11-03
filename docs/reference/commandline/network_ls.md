<!--[metadata]>
+++
title = "network ls"
description = "The network ls command description and usage"
keywords = ["network, list, user-defined"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# docker network ls

    Usage:  docker network ls [OPTIONS]

    Lists all the networks created by the user
      --help=false          Print usage
      --no-trunc=false      Do not truncate the output
      -q, --quiet=false     Only display numeric IDs

Lists all the networks the Engine `daemon` knows about. This includes the
networks that span across multiple hosts in a cluster, for example:

```bash
    $ sudo docker network ls
    NETWORK ID          NAME                DRIVER
    7fca4eb8c647        bridge              bridge
    9f904ee27bf5        none                null
    cf03ee007fb4        host                host
    78b03ee04fc4        multi-host          overlay
```

Use the `--no-trunc` option to display the full network id:

```bash
docker network ls --no-trunc
NETWORK ID                                                         NAME                DRIVER
18a2866682b85619a026c81b98a5e375bd33e1b0936a26cc497c283d27bae9b3   none                null                
c288470c46f6c8949c5f7e5099b5b7947b07eabe8d9a27d79a9cbf111adcbf47   host                host                
7b369448dccbf865d397c8d2be0cda7cf7edc6b0945f77d2529912ae917a0185   bridge              bridge              
95e74588f40db048e86320c6526440c504650a1ff3e9f7d60a497c4d2163e5bd   foo                 bridge    
```


## Related information

* [network disconnect ](network_disconnect.md)
* [network connect](network_connect.md)
* [network create](network_create.md)
* [network inspect](network_inspect.md)
* [network rm](network_rm.md)
* [Understand Docker container networks](../../userguide/networking/dockernetworks.md)

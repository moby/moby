<!--[metadata]>
+++
title = "network inspect"
description = "The network inspect command description and usage"
keywords = ["network, inspect"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# network inspect

    Usage:  docker network inspect [OPTIONS] NETWORK

    Displays detailed information on a network

      --help=false       Print usage

Returns information about a network. By default, this command renders all results
in a JSON object.

Example output:

```
$ sudo docker run -itd --name=container1 busybox
f2870c98fd504370fb86e59f32cd0753b1ac9b69b7d80566ffc7192a82b3ed27

$ sudo docker run -itd --name=container2 busybox
bda12f8922785d1f160be70736f26c1e331ab8aaf8ed8d56728508f2e2fd4727

$ sudo docker network inspect bridge
{
    "name": "bridge",
    "id": "7fca4eb8c647e57e9d46c32714271e0c3f8bf8d17d346629e2820547b2d90039",
    "driver": "bridge",
    "containers": {
        "bda12f8922785d1f160be70736f26c1e331ab8aaf8ed8d56728508f2e2fd4727": {
            "endpoint": "e0ac95934f803d7e36384a2029b8d1eeb56cb88727aa2e8b7edfeebaa6dfd758",
            "mac_address": "02:42:ac:11:00:03",
            "ipv4_address": "172.17.0.3/16",
            "ipv6_address": ""
        },
        "f2870c98fd504370fb86e59f32cd0753b1ac9b69b7d80566ffc7192a82b3ed27": {
            "endpoint": "31de280881d2a774345bbfb1594159ade4ae4024ebfb1320cb74a30225f6a8ae",
            "mac_address": "02:42:ac:11:00:02",
            "ipv4_address": "172.17.0.2/16",
            "ipv6_address": ""
        }
    }
}
```

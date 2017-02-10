---
title: "network inspect"
description: "The network inspect command description and usage"
keywords: "network, inspect, user-defined"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# network inspect

```markdown
Usage:  docker network inspect [OPTIONS] NETWORK [NETWORK...]

Display detailed information on one or more networks

Options:
  -f, --format string   Format the output using the given Go template
      --help            Print usage
```

## Description

Returns information about one or more networks. By default, this command renders
all results in a JSON object.

## Examples

## Inspect the `bridge` network

Connect two containers to the default `bridge` network:

```bash
$ sudo docker run -itd --name=container1 busybox
f2870c98fd504370fb86e59f32cd0753b1ac9b69b7d80566ffc7192a82b3ed27

$ sudo docker run -itd --name=container2 busybox
bda12f8922785d1f160be70736f26c1e331ab8aaf8ed8d56728508f2e2fd4727
```

The `network inspect` command shows the containers, by id, in its
results. For networks backed by multi-host network driver, such as Overlay,
this command also shows the container endpoints in other hosts in the
cluster. These endpoints are represented as "ep-{endpoint-id}" in the output.
However, for swarm-scoped networks, only the endpoints that are local to the
node are shown.

You can specify an alternate format to execute a given
template for each result. Go's
[text/template](http://golang.org/pkg/text/template/) package describes all the
details of the format.

```none
$ sudo docker network inspect bridge

[
    {
        "Name": "bridge",
        "Id": "b2b1a2cba717161d984383fd68218cf70bbbd17d328496885f7c921333228b0f",
        "Created": "2016-10-19T04:33:30.360899459Z",
        "Scope": "local",
        "Driver": "bridge",
        "IPAM": {
            "Driver": "default",
            "Config": [
                {
                    "Subnet": "172.17.42.1/16",
                    "Gateway": "172.17.42.1"
                }
            ]
        },
        "Internal": false,
        "Containers": {
            "bda12f8922785d1f160be70736f26c1e331ab8aaf8ed8d56728508f2e2fd4727": {
                "Name": "container2",
                "EndpointID": "0aebb8fcd2b282abe1365979536f21ee4ceaf3ed56177c628eae9f706e00e019",
                "MacAddress": "02:42:ac:11:00:02",
                "IPv4Address": "172.17.0.2/16",
                "IPv6Address": ""
            },
            "f2870c98fd504370fb86e59f32cd0753b1ac9b69b7d80566ffc7192a82b3ed27": {
                "Name": "container1",
                "EndpointID": "a00676d9c91a96bbe5bcfb34f705387a33d7cc365bac1a29e4e9728df92d10ad",
                "MacAddress": "02:42:ac:11:00:01",
                "IPv4Address": "172.17.0.1/16",
                "IPv6Address": ""
            }
        },
        "Options": {
            "com.docker.network.bridge.default_bridge": "true",
            "com.docker.network.bridge.enable_icc": "true",
            "com.docker.network.bridge.enable_ip_masquerade": "true",
            "com.docker.network.bridge.host_binding_ipv4": "0.0.0.0",
            "com.docker.network.bridge.name": "docker0",
            "com.docker.network.driver.mtu": "1500"
        },
        "Labels": {}
    }
]
```

### Inspect a user-defined network

Create and inspect a user-defined network:

```bash
$ docker network create simple-network

69568e6336d8c96bbf57869030919f7c69524f71183b44d80948bd3927c87f6a
```

```none
$ docker network inspect simple-network

[
    {
        "Name": "simple-network",
        "Id": "69568e6336d8c96bbf57869030919f7c69524f71183b44d80948bd3927c87f6a",
        "Created": "2016-10-19T04:33:30.360899459Z",
        "Scope": "local",
        "Driver": "bridge",
        "IPAM": {
            "Driver": "default",
            "Config": [
                {
                    "Subnet": "172.22.0.0/16",
                    "Gateway": "172.22.0.1"
                }
            ]
        },
        "Containers": {},
        "Options": {},
        "Labels": {}
    }
]
```

### Inspect the `ingress` network

For swarm mode overlay networks `network inspect` also shows the IP address and node name
of the peers. Peers are the nodes in the swarm cluster which have at least one task attached
to the network. Node name is of the format `<hostname>-<unique ID>`.

```none
$ docker network inspect ingress

[
    {
        "Name": "ingress",
        "Id": "j0izitrut30h975vk4m1u5kk3",
        "Created": "2016-11-08T06:49:59.803387552Z",
        "Scope": "swarm",
        "Driver": "overlay",
        "EnableIPv6": false,
        "IPAM": {
            "Driver": "default",
            "Options": null,
            "Config": [
                {
                    "Subnet": "10.255.0.0/16",
                    "Gateway": "10.255.0.1"
                }
            ]
        },
        "Internal": false,
        "Attachable": false,
        "Containers": {
            "ingress-sbox": {
                "Name": "ingress-endpoint",
                "EndpointID": "40e002d27b7e5d75f60bc72199d8cae3344e1896abec5eddae9743755fe09115",
                "MacAddress": "02:42:0a:ff:00:03",
                "IPv4Address": "10.255.0.3/16",
                "IPv6Address": ""
            }
        },
        "Options": {
            "com.docker.network.driver.overlay.vxlanid_list": "256"
        },
        "Labels": {},
        "Peers": [
            {
                "Name": "net-1-1d22adfe4d5c",
                "IP": "192.168.33.11"
            },
            {
                "Name": "net-2-d55d838b34af",
                "IP": "192.168.33.12"
            },
            {
                "Name": "net-3-8473f8140bd9",
                "IP": "192.168.33.13"
            }
        ]
    }
]
```

## Related commands

* [network disconnect ](network_disconnect.md)
* [network connect](network_connect.md)
* [network create](network_create.md)
* [network ls](network_ls.md)
* [network rm](network_rm.md)
* [network prune](network_prune.md)
* [Understand Docker container networks](https://docs.docker.com/engine/userguide/networking/)

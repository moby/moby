---
title: "node inspect"
description: "The node inspect command description and usage"
keywords: ["node, inspect"]
---

# node inspect

```markdown
Usage:  docker node inspect [OPTIONS] self|NODE [NODE...]

Display detailed information on one or more nodes

Options:
  -f, --format string   Format the output using the given go template
      --help            Print usage
      --pretty          Print the information in a human friendly format.
```

Returns information about a node. By default, this command renders all results
in a JSON array. You can specify an alternate format to execute a
given template for each result. Go's
[text/template](http://golang.org/pkg/text/template/) package describes all the
details of the format.

Example output:

    $ docker node inspect swarm-manager
    [
    {
        "ID": "e216jshn25ckzbvmwlnh5jr3g",
        "Version": {
            "Index": 10
        },
        "CreatedAt": "2016-06-16T22:52:44.9910662Z",
        "UpdatedAt": "2016-06-16T22:52:45.230878043Z",
        "Spec": {
            "Role": "manager",
            "Availability": "active"
        },
        "Description": {
            "Hostname": "swarm-manager",
            "Platform": {
                "Architecture": "x86_64",
                "OS": "linux"
            },
            "Resources": {
                "NanoCPUs": 1000000000,
                "MemoryBytes": 1039843328
            },
            "Engine": {
                "EngineVersion": "1.12.0",
                "Plugins": [
                    {
                        "Type": "Volume",
                        "Name": "local"
                    },
                    {
                        "Type": "Network",
                        "Name": "overlay"
                    },
                    {
                        "Type": "Network",
                        "Name": "null"
                    },
                    {
                        "Type": "Network",
                        "Name": "host"
                    },
                    {
                        "Type": "Network",
                        "Name": "bridge"
                    },
                    {
                        "Type": "Network",
                        "Name": "overlay"
                    }
                ]
            }
        },
        "Status": {
            "State": "ready"
        },
        "ManagerStatus": {
            "Leader": true,
            "Reachability": "reachable",
            "Addr": "168.0.32.137:2377"
        }
    }
    ]

    $ docker node inspect --format '{{ .ManagerStatus.Leader }}' self
    false

    $ docker node inspect --pretty self
    ID:                     e216jshn25ckzbvmwlnh5jr3g
    Hostname:               swarm-manager
    Joined at:              2016-06-16 22:52:44.9910662 +0000 utc
    Status:
     State:                 Ready
     Availability:          Active
    Manager Status:
     Address:               172.17.0.2:2377
     Raft Status:           Reachable
     Leader:                Yes
    Platform:
     Operating System:      linux
     Architecture:          x86_64
    Resources:
     CPUs:                  4
     Memory:                7.704 GiB
    Plugins:
      Network:              overlay, bridge, null, host, overlay
      Volume:               local
    Engine Version:         1.12.0

## Related information

* [node update](node_update.md)
* [node ps](node_ps.md)
* [node ls](node_ls.md)
* [node rm](node_rm.md)

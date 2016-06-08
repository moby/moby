<!--[metadata]>
+++
title = "node inspect"
description = "The node inspect command description and usage"
keywords = ["node, inspect"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

**Warning:** this command is part of the Swarm management feature introduced in Docker 1.12, and might be subject to non backward-compatible changes.

# node inspect

    Usage: docker node inspect [OPTIONS] self|NODE [NODE...]

    Return low-level information on a volume

      -f, --format=       Format the output using the given go template.
      --help              Print usage

Returns information about a node. By default, this command renders all results
in a JSON array. You can specify an alternate format to execute a
given template for each result. Go's
[text/template](http://golang.org/pkg/text/template/) package describes all the
details of the format.

Example output:

    $ docker node inspect swarm-manager
    [
      {
        "ID": "0gac67oclbxq7",
        "Version": {
            "Index": 2028
        },
        "CreatedAt": "2016-06-06T20:49:32.720047494Z",
        "UpdatedAt": "2016-06-07T00:23:31.207632893Z",
        "Spec": {
            "Role": "MANAGER",
            "Membership": "ACCEPTED",
            "Availability": "ACTIVE"
        },
        "Description": {
            "Hostname": "swarm-manager",
            "Platform": {
                "Architecture": "x86_64",
                "OS": "linux"
            },
            "Resources": {
                "NanoCPUs": 1000000000,
                "MemoryBytes": 1044250624
            },
            "Engine": {
                "EngineVersion": "1.12.0-dev",
                "Labels": {
                    "provider": "virtualbox"
                }
            }
        },
        "Status": {
            "State": "READY"
        },
        "Manager": {
            "Raft": {
                "RaftID": 2143745093569717375,
                "Addr": "192.168.99.118:4500",
                "Status": {
                    "Leader": true,
                    "Reachability": "REACHABLE"
                }
            }
        },
        "Attachment": {},
      }
    ]

    $ docker node inspect --format '{{ .Manager.Raft.Status.Leader }}' self
    false

## Related information

* [node update](node_update.md)
* [node tasks](node_tasks.md)
* [node ls](node_ls.md)
* [node rm](node_rm.md)

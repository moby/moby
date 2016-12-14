---
title: "service inspect"
description: "The service inspect command description and usage"
keywords: "service, inspect"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# service inspect

```Markdown
Usage:  docker service inspect [OPTIONS] SERVICE [SERVICE...]

Display detailed information on one or more services

Options:
  -f, --format string   Format the output using the given Go template
      --help            Print usage
      --pretty          Print the information in a human friendly format.
```


Inspects the specified service. This command has to be run targeting a manager
node.

By default, this renders all results in a JSON array. If a format is specified,
the given template will be executed for each result.

Go's [text/template](http://golang.org/pkg/text/template/) package
describes all the details of the format.

## Examples

### Inspecting a service  by name or ID

You can inspect a service, either by its *name*, or *ID*

For example, given the following service;

```bash
$ docker service ls
ID            NAME   MODE        REPLICAS  IMAGE
dmu1ept4cxcf  redis  replicated  3/3       redis:3.0.6
```

Both `docker service inspect redis`, and `docker service inspect dmu1ept4cxcf`
produce the same result:

```bash
$ docker service inspect redis
[
    {
        "ID": "dmu1ept4cxcfe8k8lhtux3ro3",
        "Version": {
            "Index": 12
        },
        "CreatedAt": "2016-06-17T18:44:02.558012087Z",
        "UpdatedAt": "2016-06-17T18:44:02.558012087Z",
        "Spec": {
            "Name": "redis",
            "TaskTemplate": {
                "ContainerSpec": {
                    "Image": "redis:3.0.6"
                },
                "Resources": {
                    "Limits": {},
                    "Reservations": {}
                },
                "RestartPolicy": {
                    "Condition": "any",
                    "MaxAttempts": 0
                },
                "Placement": {}
            },
            "Mode": {
                "Replicated": {
                    "Replicas": 1
                }
            },
            "UpdateConfig": {},
            "EndpointSpec": {
                "Mode": "vip"
            }
        },
        "Endpoint": {
            "Spec": {}
        }
    }
]
```

```bash
$ docker service inspect dmu1ept4cxcf
[
    {
        "ID": "dmu1ept4cxcfe8k8lhtux3ro3",
        "Version": {
            "Index": 12
        },
        ...
    }
]
```

### Inspect a service using pretty-print

You can print the inspect output in a human-readable format instead of the default
JSON output, by using the `--pretty` option:

```bash
$ docker service inspect --pretty frontend
ID:		c8wgl7q4ndfd52ni6qftkvnnp
Name:		frontend
Labels:
 - org.example.projectname=demo-app
Service Mode:	REPLICATED
 Replicas:		5
Placement:
UpdateConfig:
 Parallelism:	0
ContainerSpec:
 Image:		nginx:alpine
Resources:
Endpoint Mode:  vip
Ports:
 Name =
 Protocol = tcp
 TargetPort = 443
 PublishedPort = 4443
```

You can also use `--format pretty` for the same effect.


### Finding the number of tasks running as part of a service

The `--format` option can be used to obtain specific information about a
service. For example, the following command outputs the number of replicas
of the "redis" service.

```bash{% raw %}
$ docker service inspect --format='{{.Spec.Mode.Replicated.Replicas}}' redis
10
{% endraw %}```


## Related information

* [service create](service_create.md)
* [service logs](service_logs.md)
* [service ls](service_ls.md)
* [service rm](service_rm.md)
* [service scale](service_scale.md)
* [service ps](service_ps.md)
* [service update](service_update.md)

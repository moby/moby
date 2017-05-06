---
title: "secret inspect"
description: "The secret inspect command description and usage"
keywords: ["secret, inspect"]
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# secret inspect

```Markdown
Usage:  docker secret inspect [OPTIONS] SECRET [SECRET...]

Display detailed information on one or more secrets

Options:
  -f, --format string   Format the output using the given Go template
      --help            Print usage
```

## Description

Inspects the specified secret. This command has to be run targeting a manager
node.

By default, this renders all results in a JSON array. If a format is specified,
the given template will be executed for each result.

Go's [text/template](http://golang.org/pkg/text/template/) package
describes all the details of the format.

For detailed information about using secrets, refer to [manage sensitive data with Docker secrets](https://docs.docker.com/engine/swarm/secrets/).

## Examples

### Inspect a secret by name or ID

You can inspect a secret, either by its *name*, or *ID*

For example, given the following secret:

```bash
$ docker secret ls

ID                          NAME                CREATED             UPDATED
eo7jnzguqgtpdah3cm5srfb97   my_secret           3 minutes ago       3 minutes ago
```

```none
$ docker secret inspect secret.json

[
    {
        "ID": "eo7jnzguqgtpdah3cm5srfb97",
        "Version": {
            "Index": 17
        },
        "CreatedAt": "2017-03-24T08:15:09.735271783Z",
        "UpdatedAt": "2017-03-24T08:15:09.735271783Z",
        "Spec": {
            "Name": "my_secret",
            "Labels": {
                "env": "dev",
                "rev": "20170324"
            }
        }
    }
]
```

### Formatting

You can use the --format option to obtain specific information about a
secret. The following example command outputs the creation time of the
secret.

```bash
$ docker secret inspect --format='{{.CreatedAt}}' eo7jnzguqgtpdah3cm5srfb97

2017-03-24 08:15:09.735271783 +0000 UTC
```


## Related commands

* [secret create](secret_create.md)
* [secret ls](secret_ls.md)
* [secret rm](secret_rm.md)

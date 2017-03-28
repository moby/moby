---
title: "secret create"
description: "The secret create command description and usage"
keywords: ["secret, create"]
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# secret create

```Markdown
Usage:	docker secret create [OPTIONS] SECRET file|-

Create a secret from a file or STDIN as content

Options:
      --help          Print usage
  -l, --label list    Secret labels (default [])
```

## Description

Creates a secret using standard input or from a file for the secret content. You must run this command on a manager node. 

For detailed information about using secrets, refer to [manage sensitive data with Docker secrets](https://docs.docker.com/engine/swarm/secrets/).

## Examples

### Create a secret

```bash
$ echo <secret> | docker secret create my_secret -

onakdyv307se2tl7nl20anokv

$ docker secret ls

ID                          NAME                CREATED             UPDATED
onakdyv307se2tl7nl20anokv   my_secret           6 seconds ago       6 seconds ago
```

### Create a secret with a file

```bash
$ docker secret create my_secret ./secret.json

dg426haahpi5ezmkkj5kyl3sn

$ docker secret ls

ID                          NAME                CREATED             UPDATED
dg426haahpi5ezmkkj5kyl3sn   my_secret           7 seconds ago       7 seconds ago
```

### Create a secret with labels

```bash
$ docker secret create --label env=dev \
                       --label rev=20170324 \
                       my_secret ./secret.json

eo7jnzguqgtpdah3cm5srfb97
```

```none
$ docker secret inspect my_secret

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


## Related commands

* [secret inspect](secret_inspect.md)
* [secret ls](secret_ls.md)
* [secret rm](secret_rm.md)

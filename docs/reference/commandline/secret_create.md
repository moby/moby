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
Usage:  docker secret create [NAME]

Create a secret using stdin as content
```

Creates a secret using standard input for the secret content. You must run this
command on a manager node.

## Examples

### Create a secret

```bash
$ cat ssh-dev | docker secret create ssh-dev
mhv17xfe3gh6xc4rij5orpfds

$ docker secret ls
ID                          NAME                CREATED                                   UPDATED                                   SIZE
mhv17xfe3gh6xc4rij5orpfds   ssh-dev             2016-10-27 23:25:43.909181089 +0000 UTC   2016-10-27 23:25:43.909181089 +0000 UTC   1679
```

## Related information

* [secret inspect](secret_inspect.md)
* [secret ls](secret_ls.md)
* [secret rm](secret_rm.md)

<style>table tr > td:first-child { white-space: nowrap;}</style>

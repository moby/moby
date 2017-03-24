---
title: "volume inspect"
description: "The volume inspect command description and usage"
keywords: "volume, inspect"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# volume inspect

```markdown
Usage:  docker volume inspect [OPTIONS] VOLUME [VOLUME...]

Display detailed information on one or more volumes

Options:
  -f, --format string   Format the output using the given Go template
      --help            Print usage
```

## Description

Returns information about a volume. By default, this command renders all results
in a JSON array. You can specify an alternate format to execute a
given template for each result. Go's
[text/template](http://golang.org/pkg/text/template/) package describes all the
details of the format.

## Examples

```bash
$ docker volume create
85bffb0677236974f93955d8ecc4df55ef5070117b0e53333cc1b443777be24d
$ docker volume inspect 85bffb0677236974f93955d8ecc4df55ef5070117b0e53333cc1b443777be24d
[
  {
      "Name": "85bffb0677236974f93955d8ecc4df55ef5070117b0e53333cc1b443777be24d",
      "Driver": "local",
      "Mountpoint": "/var/lib/docker/volumes/85bffb0677236974f93955d8ecc4df55ef5070117b0e53333cc1b443777be24d/_data",
      "Status": null
  }
]

$ docker volume inspect --format '{{ .Mountpoint }}' 85bffb0677236974f93955d8ecc4df55ef5070117b0e53333cc1b443777be24d
/var/lib/docker/volumes/85bffb0677236974f93955d8ecc4df55ef5070117b0e53333cc1b443777be24d/_data
```

## Related commands

* [volume create](volume_create.md)
* [volume ls](volume_ls.md)
* [volume rm](volume_rm.md)
* [volume prune](volume_prune.md)
* [Understand Data Volumes](https://docs.docker.com/engine/tutorials/dockervolumes/)

---
title: "volume rm"
description: "the volume rm command description and usage"
keywords: "volume, rm"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# volume rm

```markdown
Usage:  docker volume rm [OPTIONS] VOLUME [VOLUME...]

Remove one or more volumes

Aliases:
  rm, remove

Options:
  -f, --force  Force the removal of one or more volumes
      --help   Print usage
```

## Description

Remove one or more volumes. You cannot remove a volume that is in use by a container.

## Examples

```bash
  $ docker volume rm hello
  hello
```

## Related commands

* [volume create](volume_create.md)
* [volume inspect](volume_inspect.md)
* [volume ls](volume_ls.md)
* [volume prune](volume_prune.md)
* [Understand Data Volumes](https://docs.docker.com/engine/tutorials/dockervolumes/)

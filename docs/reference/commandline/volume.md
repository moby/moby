---
title: "volume"
description: "The volume command description and usage"
keywords: "volume"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# volume

```markdown
Usage:  docker volume COMMAND

Manage volumes

Options:
      --help   Print usage

Commands:
  create      Create a volume
  inspect     Display detailed information on one or more volumes
  ls          List volumes
  prune       Remove all unused volumes
  rm          Remove one or more volumes

Run 'docker volume COMMAND --help' for more information on a command.
```

## Description

Manage volumes. You can use subcommands to create, inspect, list, remove, or
prune volumes.

## Related commands

* [volume create](volume_create.md)
* [volume inspect](volume_inspect.md)
* [volume list](volume_list.md)
* [volume rm](volume_rm.md)
* [volume prune](volume_prune.md)
* [Understand Data Volumes](https://docs.docker.com/engine/tutorials/dockervolumes/)

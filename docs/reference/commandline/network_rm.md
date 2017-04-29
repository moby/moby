---
title: "network rm"
description: "the network rm command description and usage"
keywords: "network, rm, user-defined"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# network rm

```markdown
Usage:  docker network rm NETWORK [NETWORK...]

Remove one or more networks

Aliases:
  rm, remove

Options:
      --help   Print usage
```

## Description

Removes one or more networks by name or identifier. To remove a network,
you must first disconnect any containers connected to it.

## Examples

### Remove a network

To remove the network named 'my-network':

```bash
  $ docker network rm my-network
```

### Remove multiple networks

To delete multiple networks in a single `docker network rm` command, provide
multiple network names or ids. The following example deletes a network with id
`3695c422697f` and a network named `my-network`:

```bash
  $ docker network rm 3695c422697f my-network
```

When you specify multiple networks, the command attempts to delete each in turn.
If the deletion of one network fails, the command continues to the next on the
list and tries to delete that. The command reports success or failure for each
deletion.

## Related commands

* [network disconnect ](network_disconnect.md)
* [network connect](network_connect.md)
* [network create](network_create.md)
* [network ls](network_ls.md)
* [network inspect](network_inspect.md)
* [network prune](network_prune.md)
* [Understand Docker container networks](https://docs.docker.com/engine/userguide/networking/)

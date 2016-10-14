---
title: "volume rm"
description: "the volume rm command description and usage"
keywords: ["volume, rm"]
---


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

Remove one or more volumes. You cannot remove a volume that is in use by a container.

    $ docker volume rm hello
    hello

## Related information

* [volume create](volume_create.md)
* [volume inspect](volume_inspect.md)
* [volume ls](volume_ls.md)
* [Understand Data Volumes](../../tutorials/dockervolumes.md)

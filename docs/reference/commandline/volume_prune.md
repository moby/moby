---
title: "volume prune"
description: "Remove unused volumes"
keywords: [volume, prune, delete]
---

# volume prune

```markdown
Usage:	docker volume prune [OPTIONS]

Remove all unused volumes

Options:
  -f, --force   Do not prompt for confirmation
      --help    Print usage
```

Remove all unused volumes. Unused volumes are those which are not referenced by any containers

Example output:

```bash
$ docker volume prune
WARNING! This will remove all volumes not used by at least one container.
Are you sure you want to continue? [y/N] y
Deleted Volumes:
07c7bdf3e34ab76d921894c2b834f073721fccfbbcba792aa7648e3a7a664c2e
my-named-vol

Total reclaimed space: 36 B
```

## Related information

* [volume create](volume_create.md)
* [volume ls](volume_ls.md)
* [volume inspect](volume_inspect.md)
* [volume rm](volume_rm.md)
* [Understand Data Volumes](../../tutorials/dockervolumes.md)
* [system df](system_df.md)
* [container prune](container_prune.md)
* [image prune](image_prune.md)
* [system prune](system_prune.md)

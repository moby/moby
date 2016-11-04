---
title: "network prune"
description: "Remove unused networks"
keywords: "network, prune, delete"
---

# network prune

```markdown
Usage:	docker network prune [OPTIONS]

Remove all unused networks

Options:
  -f, --force   Do not prompt for confirmation
      --help    Print usage
```

Remove all unused networks. Unused networks are those which are not referenced by any containers.

Example output:

```bash
$ docker network prune
WARNING! This will remove all networks not used by at least one container.
Are you sure you want to continue? [y/N] y
Deleted Networks:
n1
n2
```

## Related information

* [network disconnect ](network_disconnect.md)
* [network connect](network_connect.md)
* [network create](network_create.md)
* [network ls](network_ls.md)
* [network inspect](network_inspect.md)
* [network rm](network_rm.md)
* [Understand Docker container networks](https://docs.docker.com/engine/userguide/networking/)
* [system df](system_df.md)
* [container prune](container_prune.md)
* [image prune](image_prune.md)
* [volume prune](volume_prune.md)
* [system prune](system_prune.md)

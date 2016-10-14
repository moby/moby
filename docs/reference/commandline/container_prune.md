---
title: "container prune"
description: "Remove all stopped containers"
keywords: [container, prune, delete, remove]
---

# container prune

```markdown
Usage:	docker container prune [OPTIONS]

Remove all stopped containers

Options:
  -f, --force   Do not prompt for confirmation
      --help    Print usage
```

## Examples

```bash
$ docker container prune
WARNING! This will remove all stopped containers.
Are you sure you want to continue? [y/N] y
Deleted Containers:
4a7f7eebae0f63178aff7eb0aa39cd3f0627a203ab2df258c1a00b456cf20063
f98f9c2aa1eaf727e4ec9c0283bc7d4aa4762fbdba7f26191f26c97f64090360

Total reclaimed space: 212 B
```

## Related information

* [system df](system_df.md)
* [volume prune](volume_prune.md)
* [image prune](image_prune.md)
* [system prune](system_prune.md)

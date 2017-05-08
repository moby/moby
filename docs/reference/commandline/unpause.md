---
redirect_from:
  - /reference/commandline/unpause/
description: The unpause command description and usage
keywords:
- cgroups, suspend, container
title: docker unpause
---

```markdown
Usage:  docker unpause CONTAINER [CONTAINER...]

Unpause all processes within one or more containers

Options:
      --help   Print usage
```

The `docker unpause` command uses the cgroups freezer to un-suspend all
processes in a container.

See the
[cgroups freezer documentation](https://www.kernel.org/doc/Documentation/cgroup-v1/freezer-subsystem.txt)
for further details.

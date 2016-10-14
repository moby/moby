---
title: "unpause"
description: "The unpause command description and usage"
keywords: ["cgroups, suspend, container"]
---

# unpause

```markdown
Usage:  docker unpause CONTAINER [CONTAINER...]

Unpause all processes within one or more containers

Options:
      --help   Print usage
```

The `docker unpause` command un-suspends all processes in a container.
On Linux, it does this using the cgroups freezer.

See the
[cgroups freezer documentation](https://www.kernel.org/doc/Documentation/cgroup-v1/freezer-subsystem.txt)
for further details.

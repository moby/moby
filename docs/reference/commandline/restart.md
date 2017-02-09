---
title: "restart"
description: "The restart command description and usage"
keywords: "restart, container, Docker"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# restart

```markdown
Usage:  docker restart [OPTIONS] CONTAINER [CONTAINER...]

Restart one or more containers

Options:
      --help       Print usage
  -t, --time int   Seconds to wait for stop before killing the container (default 10)
```

## Examples

```bash
$ docker restart my_container
```

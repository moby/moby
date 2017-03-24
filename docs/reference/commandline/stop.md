---
title: "stop"
description: "The stop command description and usage"
keywords: "stop, SIGKILL, SIGTERM"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# stop

```markdown
Usage:  docker stop [OPTIONS] CONTAINER [CONTAINER...]

Stop one or more running containers

Options:
      --help       Print usage
  -t, --time int   Seconds to wait for stop before killing it (default 10)
```

## Description

The main process inside the container will receive `SIGTERM`, and after a grace
period, `SIGKILL`.

## Examples

```bash
$ docker stop my_container
```

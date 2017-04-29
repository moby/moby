---
title: "wait"
description: "The wait command description and usage"
keywords: "container, stop, wait"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# wait

```markdown
Usage:  docker wait CONTAINER [CONTAINER...]

Block until one or more containers stop, then print their exit codes

Options:
      --help   Print usage
```

> **Note**: `docker wait` returns `0` when run against a container which had
> already exited before the `docker wait` command was run.

## Examples

Start a container in the background.

```bash
$ docker run -dit --name=my_container ubuntu bash
```

Run `docker wait`, which should block until the container exits.

```bash
$ docker wait my_container
```

In another terminal, stop the first container. The `docker wait` command above
returns the exit code.

```bash
$ docker stop my_container
```

This is the same `docker wait` command from above, but it now exits, returning
`0`.

```bash
$ docker wait my_container

0
```

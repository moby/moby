---
title: "save"
description: "The save command description and usage"
keywords: "tarred, repository, backup"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# save

```markdown
Usage:  docker save [OPTIONS] IMAGE [IMAGE...]

Save one or more images to a tar archive (streamed to STDOUT by default)

Options:
      --help            Print usage
  -o, --output string   Write to a file, instead of STDOUT
```

## Description

Produces a tarred repository to the standard output stream.
Contains all parent layers, and all tags + versions, or specified `repo:tag`, for
each argument provided.

## Examples

### Create a backup that can then be used with `docker load`.

```bash
$ docker save busybox > busybox.tar

$ ls -sh busybox.tar

2.7M busybox.tar

$ docker save --output busybox.tar busybox

$ ls -sh busybox.tar

2.7M busybox.tar

$ docker save -o fedora-all.tar fedora

$ docker save -o fedora-latest.tar fedora:latest
```

### Cherry-pick particular tags

You can even cherry-pick particular tags of an image repository.

```bash
$ docker save -o ubuntu.tar ubuntu:lucid ubuntu:saucy
```

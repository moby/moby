---
title: "rm"
description: "The rm command description and usage"
keywords: "remove, Docker, container"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# rm

```markdown
Usage:  docker rm [OPTIONS] CONTAINER [CONTAINER...]

Remove one or more containers

Options:
  -f, --force     Force the removal of a running container (uses SIGKILL)
      --help      Print usage
  -l, --link      Remove the specified link
  -v, --volumes   Remove the volumes associated with the container
```

## Examples

### Remove a container

This will remove the container referenced under the link
`/redis`.

```bash
$ docker rm /redis

/redis
```

### Remove a link specified with `--link` on the default bridge network

This will remove the underlying link between `/webapp` and the `/redis`
containers on the default bridge network, removing all network communication
between the two containers. This does not apply when `--link` is used with
user-specified networks.

```bash
$ docker rm --link /webapp/redis

/webapp/redis
```

### Force-remove a running container

This command will force-remove a running container.

```bash
$ docker rm --force redis

redis
```

The main process inside the container referenced under the link `redis` will receive
`SIGKILL`, then the container will be removed.

### Remove all stopped containers

```bash
$ docker rm $(docker ps -a -q)
```

This command will delete all stopped containers. The command
`docker ps -a -q` will return all existing container IDs and pass them to
the `rm` command which will delete them. Any running containers will not be
deleted.

### Remove a container and its volumes

```bash
$ docker rm -v redis
redis
```

This command will remove the container and any volumes associated with it.
Note that if a volume was specified with a name, it will not be removed.

### Remove a container and selectively remove volumes

```bash
$ docker create -v awesome:/foo -v /bar --name hello redis
hello
$ docker rm -v hello
```

In this example, the volume for `/foo` will remain intact, but the volume for
`/bar` will be removed. The same behavior holds for volumes inherited with
`--volumes-from`.

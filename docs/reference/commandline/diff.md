---
title: "diff"
description: "The diff command description and usage"
keywords: "list, changed, files, container"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

## diff

```markdown
Usage:  docker diff CONTAINER

Inspect changes to files or directories on a container's filesystem

Options:
      --help   Print usage
```

List the changed files and directories in a container᾿s filesystem since the
container was created. Three different types of change are tracked:

| Symbol | Description                     |
|--------|---------------------------------|
| `A`    | A file or directory was added   |
| `D`    | A file or directory was deleted |
| `C`    | A file or directory was changed |

You can use the full or shortened container ID or the container name set using
`docker run --name` option.

## Examples

Inspect the changes to an `nginx` container:

```bash
$ docker diff 1fdfd1f54c1b

C /dev
C /dev/console
C /dev/core
C /dev/stdout
C /dev/fd
C /dev/ptmx
C /dev/stderr
C /dev/stdin
C /run
A /run/nginx.pid
C /var/lib/nginx/tmp
A /var/lib/nginx/tmp/client_body
A /var/lib/nginx/tmp/fastcgi
A /var/lib/nginx/tmp/proxy
A /var/lib/nginx/tmp/scgi
A /var/lib/nginx/tmp/uwsgi
C /var/log/nginx
A /var/log/nginx/access.log
A /var/log/nginx/error.log
```

% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-diff - Inspect changes to files or directories on a container's filesystem

# SYNOPSIS
**docker diff**
[**--help**]
CONTAINER

# DESCRIPTION
List the changed files and directories in a container᾿s filesystem since the
container was created. Three different types of change are tracked:

| Symbol | Description                     |
|--------|---------------------------------|
| `A`    | A file or directory was added   |
| `D`    | A file or directory was deleted |
| `C`    | A file or directory was changed |

You can use the full or shortened container ID or the container name set using
**docker run --name** option.

# OPTIONS
**--help**
  Print usage statement

# EXAMPLES

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


# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>

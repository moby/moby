% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-diff - Inspect changes on a container's filesystem

# SYNOPSIS
**docker diff** CONTAINER

# DESCRIPTION
Inspect changes on a container's filesystem. You can use the full or
shortened container ID or the container name set using
**docker run --name** option.

# EXAMPLE
Inspect the changes to on a nginx container:

    # docker diff 1fdfd1f54c1b
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


# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.



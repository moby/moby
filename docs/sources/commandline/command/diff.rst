:title: Diff Command
:description: Inspect changes on a container's filesystem
:keywords: diff, docker, container, documentation

=======================================================
``diff`` -- Inspect changes on a container's filesystem
=======================================================

::

    Usage: docker diff CONTAINER

    List the changed files and directories in a container's filesystem

There are 3 events that are listed in the 'diff':
1. ```A``` - Add
2. ```D``` - Delete
3. ```C``` - Change

for example

```
# docker diff 7bb0e258aefe

C /dev
A /dev/kmsg
C /etc
A /etc/mtab
A /go
A /go/src
A /go/src/github.com
A /go/src/github.com/dotcloud
A /go/src/github.com/dotcloud/docker
A /go/src/github.com/dotcloud/docker/.git
....
```

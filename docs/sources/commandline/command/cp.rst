:title: Cp Command
:description: Copy files/folders from the containers filesystem to the host path
:keywords: cp, docker, container, documentation, copy

============================================================================
``cp`` -- Copy files/folders from the containers filesystem to the host path
============================================================================

::

    Usage: docker cp CONTAINER:PATH HOSTPATH

    Copy files/folders from the containers filesystem to the host
    path.  Paths are relative to the root of the filesystem.


For example:

```docker cp 7bb0e258aefe:/etc/debian_version .```

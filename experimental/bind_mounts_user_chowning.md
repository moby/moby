# Experimental: Bind mounts user chown'ing

The [experimental build of Docker](https://github.com/docker/docker/tree/master/experimental) introduces a new mount mode option `u` in the volume flags when the volume is a bind mount. When the container starts the daemon will recursively chown the bind path to the user specified with `--user`. Note that if `--user` isn't provided the chowning isn't performed. Trying to set `u` when the bind mount source is one `/`, `/usr` or `/etc` fails.

## EXAMPLES

    export TMPDIR=$(mktemp -d)
    docker run -v $TMPDIR:/test:u --user 1 busybox ls -la /test

    total 4
    drwx------    2 daemon   daemon          40 Oct  5 19:04 .
    drwxr-xr-x    1 root     root          4096 Oct  5 19:05 ..


## WARNING

Using this feature will change the permissions on the host, it should be used with care.
For example:

    docker run -v ~:/home/someuser:u -u 1 busybox ls -la /home

will change the host's `/home` owner to UID `1` which could be a serious problem.

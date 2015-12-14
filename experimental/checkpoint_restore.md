# Docker Checkpoint & Restore

Checkpoint & Restore is a new feature that allows you to freeze a running
container by checkpointing it, which turns its state into a collection of files
on disk. Later, the container can be restored from the point it was frozen.

This is accomplished using a tool called [CRIU](http://criu.org), which is an
external dependency of this feature. A good overview of the history of
checkpoint and restore in Docker is available in this
[Kubernetes blog post](http://blog.kubernetes.io/2015/07/how-did-quake-demo-from-dockercon-work.html).

## Installing CRIU

If you use a Debian system, you can add the CRIU PPA and install with apt-get
https://launchpad.net/~criu/+archive/ubuntu/ppa.

Alternatively, you can [build CRIU from source](http://criu.org/Installation).

## Use cases for checkpoint & restore

This feature is currently focused on single-host use cases for checkpoint and
restore. Here are a few:

- Restarting / upgrading the docker daemon without stopping containers
- Restarting the host machine without stopping/starting containers
- Speeding up the start time of slow start applications
- "Rewinding" processes to an earlier point in time
- "Forensic debugging" of running processes

Another primary use case of checkpoint & restore outside of Docker is the live
migration of a server from one machine to another. This is possible with the
current implementation, but not currently a priority (and so the workflow is
not optimized for the task).

## Using Checkpoint & Restore

Two new top level commands are introduced in the CLI: `checkpoint` & `restore`.
The options for checkpoint:

    Usage:  docker checkpoint [OPTIONS] CONTAINER [CONTAINER...]

    Checkpoint one or more running containers

      --image-dir=             directory for storing checkpoint image files (optional)
      --work-dir=              directory for storing log file (optional)
      --leave-running=false    leave the container running after checkpoint

And for restore:

    Usage:  docker restore [OPTIONS] CONTAINER [CONTAINER...]

    Restore one or more checkpointed containers

      --image-dir=           directory to restore image files from (optional)
      --work-dir=            directory for restore log (optional)
      --force=false          bypass checks for current container state

A simple example of using checkpoint & restore on a container:

    $ docker run --name cr -d busybox /bin/sh -c 'i=0; while true; do echo $i; i=$(expr $i + 1); sleep 1; done'
    > abc0123

    $ docker checkpoint cr
    > abc0123

    $ docker restore cr
    > abc0123

This process just logs an incrementing counter to stdout. If you `docker logs`
in between running/checkpoint/restoring you should see that the counter
increases while the process is running, stops while it's checkpointed, and
resumes from the point it left off once you restore.

### Same container checkpoint/restore

The above example falls into the category of "same container" use cases for c/r.
Restarting the daemon is an example of this kind of use case. There is only one
container here at any point in time. That container's status, once it is
checkpointed, will be "Checkpointed" and docker inspect will contain that status
as well as the time of the last checkpoint. The IP address and other container
state do not change (see known issues at the bottom of this document).

### New container checkpoint/restore

Here's an example of a "new container" use case for c/r:

    $ docker run some_image
    > abc789

    ## the container runs for a while

    $ docker checkpoint --image-dir=/some/path abc789
    > abc789

At this point, we've created a checkpoint image at `/some/path` that encodes a
process at the exact state we want it to be. Now, at some later point in time,
we can put a copy of that exact state into a new container (perhaps many times):

    $ docker create some_image
    > def123

    $ docker restore --force=true --image-dir=/some/path def123
    > def123

We created a new container (but didn't start it), and then we restored our
checkpointed process into that container.

This is obviously more involved than the simple use case shown earlier. It
requires starting subsequent containers with the same configuration (e.g.
the same mounted volumes, the same base image, etc.). Specifically, it should
be noted that checkpoints do not capture any changes to the filesystem, so it's
likely that a separate docker commit should be used to capture the changed
filesystem and use that when creating the new container to restore into.

### Options

Checkpoint & Restore:

      --image-dir=             directory for storing checkpoint image files

Allows you to specify the path for writing a checkpoint image, or the path for
the image you want to restore. Defaults to the internal docker container dir.

      --work-dir=              directory for storing log file

Allows you to specify the path for writing the CRIU log. Defaults to the
internal docker container dir.

      --leave-running=false    leave the container running after checkpoint

Normally, when checkpointing a process, the process is stopped aftewrards.
When this flag is enabled, the process keeps running after a checkpoint. This is
useful if you want to capture a process at multiple points in time, for later
use in debugging or rewinding a process for some reason. It's also used for
minimizing downtime when checkpointing processes with a large memory footprint.

Restore Only:

      --force=false            force restoring into a container

As shown in the "new container" example, this flag allows you to restore a
checkpoint image into a container that was not previously checkpointed.
Normally, docker would return an error when restoring into a container that
has not been previously checkpointed.

## Known Issues

- Currently, networking is broken in this PR. Although it's implemented at the
libcontainer level, the method used no longer works since the introduction of
libnetwork. See:
    - https://github.com/docker/libnetwork/pull/465
    - https://github.com/boucher/docker/pull/15
- There are likely several networking related issues to work out, like:
    - ensuring IPs are reserved across daemon restarts
    - ensuring port maps are reserved
    - deciding how to deal with network resources in the "new container" model

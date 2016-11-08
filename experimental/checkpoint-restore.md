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
[from the criu launchpad](https://launchpad.net/~criu/+archive/ubuntu/ppa).

Alternatively, you can [build CRIU from source](http://criu.org/Installation).

You need at least version 2.0 of CRIU to run checkpoint/restore in Docker.

## Use cases for checkpoint & restore

This feature is currently focused on single-host use cases for checkpoint and
restore. Here are a few:

- Restarting the host machine without stopping/starting containers
- Speeding up the start time of slow start applications
- "Rewinding" processes to an earlier point in time
- "Forensic debugging" of running processes

Another primary use case of checkpoint & restore outside of Docker is the live
migration of a server from one machine to another. This is possible with the
current implementation, but not currently a priority (and so the workflow is
not optimized for the task).

## Using checkpoint & restore

A new top level command `docker checkpoint` is introduced, with three subcommands:
- `create` (creates a new checkpoint)
- `ls` (lists existing checkpoints)
- `rm` (deletes an existing checkpoint)

Additionally, a `--checkpoint` flag is added to the container start command.

The options for checkpoint create:

    Usage:  docker checkpoint create [OPTIONS] CONTAINER CHECKPOINT

    Create a checkpoint from a running container

      --leave-running=false    Leave the container running after checkpoint
      --checkpoint-dir         Use a custom checkpoint storage directory

And to restore a container:

    Usage:  docker start --checkpoint CHECKPOINT_ID [OTHER OPTIONS] CONTAINER


A simple example of using checkpoint & restore on a container:

    $ docker run --security-opt=seccomp:unconfined --name cr -d busybox /bin/sh -c 'i=0; while true; do echo $i; i=$(expr $i + 1); sleep 1; done'
    > abc0123

    $ docker checkpoint create cr checkpoint1

    # <later>
    $ docker start --checkpoint checkpoint1 cr
    > abc0123

This process just logs an incrementing counter to stdout. If you `docker logs`
in between running/checkpoint/restoring you should see that the counter
increases while the process is running, stops while it's checkpointed, and
resumes from the point it left off once you restore.

## Current limitation

seccomp is only supported by CRIU in very up to date kernels.

External terminal (i.e. `docker run -t ..`) is not supported at the moment.
If you try to create a checkpoint for a container with an external terminal, 
it would fail:

    $ docker checkpoint create cr checkpoint1
    Error response from daemon: Cannot checkpoint container c1: rpc error: code = 2 desc = exit status 1: "criu failed: type NOTIFY errno 0\nlog file: /var/lib/docker/containers/eb62ebdbf237ce1a8736d2ae3c7d88601fc0a50235b0ba767b559a1f3c5a600b/checkpoints/checkpoint1/criu.work/dump.log\n"
    
    $ cat /var/lib/docker/containers/eb62ebdbf237ce1a8736d2ae3c7d88601fc0a50235b0ba767b559a1f3c5a600b/checkpoints/checkpoint1/criu.work/dump.log
    Error (mount.c:740): mnt: 126:./dev/console doesn't have a proper root mount


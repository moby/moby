<!--[metadata]>
+++
aliases = ["/engine/userguide/basics/"]
title = "Quickstart"
description = "Common usage and commands"
keywords = ["Examples, Usage, basic commands, docker, documentation,  examples"]
[menu.main]
parent = "engine_use"
weight=-90
+++
<![end-metadata]-->

# Docker Engine Quickstart

This quickstart assumes you have a working installation of Docker Engine. To verify Engine is installed and configured, use the following command:

    # Check that you have a working install
    $ docker info

If you have a successful install, the system information appears. If you get `docker: command not found` or something like
`/var/lib/docker/repositories: permission denied` you may have an
incomplete Docker installation or insufficient privileges to access
Engine on your machine. With the default installation of Engine `docker`
commands need to be run by a user that is in the `docker` group or by the
`root` user.

Depending on your Engine system configuration, you may be required
to preface each `docker` command with `sudo`. If you want to run without using
`sudo` with the `docker` commands, then create a Unix group called `docker` and
add the user to the 'docker' group.

For more information about installing Docker Engine or `sudo` configuration, refer to
the [installation](installation/index.md) instructions for your operating system.


## Download a pre-built image

To pull an `ubuntu` image, run:

    # Download an ubuntu image
    $ docker pull ubuntu

This downloads the `ubuntu` image by name from [Docker Hub](https://hub.docker.com) to a local
image cache. To search for an image, run `docker search`. For more information, go to:
[Searching images](userguide/containers/dockerrepos.md#searching-for-images)


> **Note**:
> When the image is successfully downloaded, you see a 12 character
> hash `539c0211cd76: Download complete` which is the
> short form of the Image ID. These short Image IDs are the first 12
> characters of the full Image ID. To view this information, run
> `docker inspect` or `docker images --no-trunc=true`.

To display a list of downloaded images, run `docker images`.

## Running an interactive shell

To run an interactive shell in the Ubuntu image:

    $ docker run -i -t ubuntu /bin/bash       

The `-i` flag starts an interactive container.
The `-t` flag creates a pseudo-TTY that attaches `stdin` and `stdout`.  
The image is `ubuntu`.
The command `/bin/bash` starts a shell you can log in.

To detach the `tty` without exiting the shell, use the escape sequence
`Ctrl-p` + `Ctrl-q`. The container continues to exist in a stopped state
once exited. To list all running containers, run `docker ps`. To view stopped and running containers,
run `docker ps -a`.

## Bind Docker to another host/port or a Unix socket

> **Warning**:
> Changing the default `docker` daemon binding to a
> TCP port or Unix *docker* user group will increase your security risks
> by allowing non-root users to gain *root* access on the host. Make sure
> you control access to `docker`. If you are binding
> to a TCP port, anyone with access to that port has full Docker access;
> so it is not advisable on an open network.

With `-H` it is possible to make the Docker daemon to listen on a
specific IP and port. By default, it will listen on
`unix:///var/run/docker.sock` to allow only local connections by the
*root* user. You *could* set it to `0.0.0.0:2375` or a specific host IP
to give access to everybody, but that is **not recommended** because
then it is trivial for someone to gain root access to the host where the
daemon is running.

Similarly, the Docker client can use `-H` to connect to a custom port.
The Docker client will default to connecting to `unix:///var/run/docker.sock`
on Linux, and `tcp://127.0.0.1:2376` on Windows.

`-H` accepts host and port assignment in the following format:

    tcp://[host]:[port][path] or unix://path

For example:

-   `tcp://` -> TCP connection to `127.0.0.1` on either port `2376` when TLS encryption
    is on, or port `2375` when communication is in plain text.
-   `tcp://host:2375` -> TCP connection on
    host:2375
-   `tcp://host:2375/path` -> TCP connection on
    host:2375 and prepend path to all requests
-   `unix://path/to/socket` -> Unix socket located
    at `path/to/socket`

`-H`, when empty, will default to the same value as
when no `-H` was passed in.

`-H` also accepts short form for TCP bindings:

    `host:` or `host:port` or `:port`

Run Docker in daemon mode:

    $ sudo <path to>/dockerd -H 0.0.0.0:5555 &

Download an `ubuntu` image:

    $ docker -H :5555 pull ubuntu

You can use multiple `-H`, for example, if you want to listen on both
TCP and a Unix socket

    # Run docker in daemon mode
    $ sudo <path to>/dockerd -H tcp://127.0.0.1:2375 -H unix:///var/run/docker.sock &
    # Download an ubuntu image, use default Unix socket
    $ docker pull ubuntu
    # OR use the TCP port
    $ docker -H tcp://127.0.0.1:2375 pull ubuntu

## Starting a long-running worker process

    # Start a very useful long-running process
    $ JOB=$(docker run -d ubuntu /bin/sh -c "while true; do echo Hello world; sleep 1; done")

    # Collect the output of the job so far
    $ docker logs $JOB

    # Kill the job
    $ docker kill $JOB

## Listing containers

    $ docker ps # Lists only running containers
    $ docker ps -a # Lists all containers

## Controlling containers

    # Start a new container
    $ JOB=$(docker run -d ubuntu /bin/sh -c "while true; do echo Hello world; sleep 1; done")

    # Stop the container
    $ docker stop $JOB

    # Start the container
    $ docker start $JOB

    # Restart the container
    $ docker restart $JOB

    # SIGKILL a container
    $ docker kill $JOB

    # Remove a container
    $ docker stop $JOB # Container must be stopped to remove it
    $ docker rm $JOB

## Bind a service on a TCP port

    # Bind port 4444 of this container, and tell netcat to listen on it
    $ JOB=$(docker run -d -p 4444 ubuntu:12.10 /bin/nc -l 4444)

    # Which public port is NATed to my container?
    $ PORT=$(docker port $JOB 4444 | awk -F: '{ print $2 }')

    # Connect to the public port
    $ echo hello world | nc 127.0.0.1 $PORT

    # Verify that the network connection worked
    $ echo "Daemon received: $(docker logs $JOB)"

## Committing (saving) a container state

To save the current state of a container as an image:

    $ docker commit <container> <some_name>

When you commit your container, Docker Engine only stores the diff (difference) between
the source image and the current state of the container's image. To list images
you already have, run:

    # List your images
    $ docker images

You now have an image state from which you can create new instances.

## Where to go next

* Work your way through the [Docker Engine User Guide](userguide/index.md)
* Read more about [Store Images on Docker Hub](userguide/containers/dockerrepos.md)
* Review [Command Line](reference/commandline/cli.md)

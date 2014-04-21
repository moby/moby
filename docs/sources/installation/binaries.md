page_title: Installation from Binaries
page_description: This instruction set is meant for hackers who want to try out Docker on a variety of environments.
page_keywords: binaries, installation, docker, documentation, linux

# Binaries

> **Note**:
> Docker is still under heavy development! We don’t recommend using it in
> production yet, but we’re getting closer with each release. Please see
> our blog post, [Getting to Docker 1.0](
> http://blog.docker.io/2013/08/getting-to-docker-1-0/)

**This instruction set is meant for hackers who want to try out Docker
on a variety of environments.**

Before following these directions, you should really check if a packaged
version of Docker is already available for your distribution. We have
packages for many distributions, and more keep showing up all the time!

## Check runtime dependencies

To run properly, docker needs the following software to be installed at
runtime:

-   iptables version 1.4 or later
-   Git version 1.7 or later
-   procps (or similar provider of a "ps" executable)
-   XZ Utils 4.9 or later
-   a [properly
    mounted](https://github.com/tianon/cgroupfs-mount/blob/master/cgroupfs-mount)
    cgroupfs hierarchy (having a single, all-encompassing "cgroup" mount
    point [is](https://github.com/dotcloud/docker/issues/2683)
    [not](https://github.com/dotcloud/docker/issues/3485)
    [sufficient](https://github.com/dotcloud/docker/issues/4568))

## Check kernel dependencies

Docker in daemon mode has specific kernel requirements. For details,
check your distribution in [*Installation*](../#installation-list).

In general, a 3.8 Linux kernel (or higher) is preferred, as some of the
prior versions have known issues that are triggered by Docker.

Note that Docker also has a client mode, which can run on virtually any
Linux kernel (it even builds on OSX!).

## Get the docker binary:

    wget https://get.docker.io/builds/Linux/x86_64/docker-latest -O docker
    chmod +x docker

> **Note**:
> If you have trouble downloading the binary, you can also get the smaller
> compressed release file:
> [https://get.docker.io/builds/Linux/x86\_64/docker-latest.tgz](
> https://get.docker.io/builds/Linux/x86_64/docker-latest.tgz)

## Run the docker daemon

    # start the docker in daemon mode from the directory you unpacked
    sudo ./docker -d &

## Giving non-root access

The `docker` daemon always runs as the root user,
and since Docker version 0.5.2, the `docker` daemon
binds to a Unix socket instead of a TCP port. By default that Unix
socket is owned by the user *root*, and so, by default, you can access
it with `sudo`.

Starting in version 0.5.3, if you (or your Docker installer) create a
Unix group called *docker* and add users to it, then the
`docker` daemon will make the ownership of the Unix
socket read/writable by the *docker* group when the daemon starts. The
`docker` daemon must always run as the root user,
but if you run the `docker` client as a user in the
*docker* group then you don’t need to add `sudo` to
all the client commands.

> **Warning**: 
> The *docker* group (or the group specified with `-G`) is root-equivalent;
> see [*Docker Daemon Attack Surface*](
> ../../articles/security/#dockersecurity-daemon) details.

## Upgrades

To upgrade your manual installation of Docker, first kill the docker
daemon:

    killall docker

Then follow the regular installation steps.

## Run your first container!

    # check your docker version
    sudo ./docker version

    # run a container and open an interactive shell in the container
    sudo ./docker run -i -t ubuntu /bin/bash

Continue with the [*Hello
World*](../../examples/hello_world/#hello-world) example.

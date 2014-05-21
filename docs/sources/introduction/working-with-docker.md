page_title: Introduction to working with Docker
page_description: Introduction to working with Docker and Docker commands.
page_keywords: docker, introduction, documentation, about, technology, understanding, Dockerfile

# An Introduction to working with Docker

**Getting started with Docker**

> **Note:** 
> If you would like to see how a specific command
> works, check out the glossary of all available client
> commands on our [Commands Reference](/reference/commandline/cli).

## Introduction

In the [Understanding Docker](understanding-docker.md) section we
covered the components that make up Docker, learned about the underlying
technology and saw *how* everything works.

Now, let's get an introduction to the basics of interacting with Docker.

> **Note:** 
> This page assumes you have a host with a running Docker
> daemon and access to a Docker client. To see how to install Docker on
> a variety of platforms see the [installation
> section](/installation/#installation).

## How to use the client

The client provides you a command-line interface to Docker. It is
accessed by running the `docker` binary.

> **Tip:** 
> The below instructions can be considered a summary of our
> [interactive tutorial](https://www.docker.io/gettingstarted). If you
> prefer a more hands-on approach without installing anything, why not
> give that a shot and check out the
> [tutorial](https://www.docker.io/gettingstarted).

The `docker` client usage is pretty simple. Each action you can take
with Docker is a command and each command can take a series of
flags and arguments.

    # Usage:  [sudo] docker [flags] [command] [arguments] ..
    # Example:
    $ docker run -i -t ubuntu /bin/bash

## Using the Docker client

Let's get started with the Docker client by running our first Docker
command. We're going to use the `docker version` command to return
version information on the currently installed Docker client and daemon.

    # Usage: [sudo] docker version
    # Example:
    $ docker version

This command will not only provide you the version of Docker client and
daemon you are using, but also the version of Go (the programming
language powering Docker).

    Client version: 0.8.0
    Go version (client): go1.2

    Git commit (client): cc3a8c8
    Server version: 0.8.0

    Git commit (server): cc3a8c8
    Go version (server): go1.2

    Last stable version: 0.8.0

### Seeing what the Docker client can do

We can see all of the commands available to us with the Docker client by
running the `docker` binary without any options.

    # Usage: [sudo] docker
    # Example:
    $ docker

You will see a list of all currently available commands.

    Commands:
         attach    Attach to a running container
         build     Build a container from a Dockerfile
         commit    Create a new image from a container's changes
    . . .

### Seeing Docker command usage

You can also zoom in and review the usage for specific Docker commands.

Try typing Docker followed with a `[command]` to see the usage for that
command:

    # Usage: [sudo] docker [command] [--help]
    # Example:
    $ docker attach
    Help output . . .

Or you can also pass the `--help` flag to the `docker` binary.

    $ docker images --help

This will display the help text and all available flags:

    Usage: docker attach [OPTIONS] CONTAINER

    Attach to a running container

      --no-stdin=false: Do not attach stdin
      --sig-proxy=true: Proxify all received signal to the process (even in non-tty mode)

## Working with images

Let's get started with using Docker by working with Docker images, the
building blocks of Docker containers.

### Docker Images

As we've discovered a Docker image is a read-only template that we build
containers from. Every Docker container is launched from an image. You can
use both images provided by Docker, such as the base `ubuntu` image,
as well as images built by others. For example we can build an image that
runs Apache and our own web application as a starting point to launch containers.

### Searching for images

To search for Docker image we use the `docker search` command. The
`docker search` command returns a list of all images that match your
search criteria, together with some useful information about that image.

This information includes social metrics like how many other people like
the image: we call these "likes" *stars*. We also tell you if an image
is *trusted*. A *trusted* image is built from a known source and allows
you to introspect in greater detail how the image is constructed.

    # Usage: [sudo] docker search [image name]
    # Example:
    $ docker search nginx

    NAME                               DESCRIPTION                                     STARS  OFFICIAL   TRUSTED
    dockerfile/nginx                   Trusted Nginx (http://nginx.org/) Build         6                 [OK]
    paintedfox/nginx-php5              A docker image for running Nginx with PHP5.     3                 [OK]
    dockerfiles/django-uwsgi-nginx     Dockerfile and configuration files to buil...   2                 [OK]
    . . .

> **Note:** 
> To learn more about trusted builds, check out
> [this](http://blog.docker.io/2013/11/introducing-trusted-builds) blog
> post.

### Downloading an image

Once we find an image we'd like to download we can pull it down from
[Docker.io](https://index.docker.io) using the `docker pull` command.

    # Usage: [sudo] docker pull [image name]
    # Example:
    $ docker pull dockerfile/nginx

    Pulling repository dockerfile/nginx
    0ade68db1d05: Pulling dependent layers
    27cf78414709: Download complete
    b750fe79269d: Download complete
    . . .

As you can see, Docker will download, one by one, all the layers forming
the image.

### Listing available images

You may already have some images you've pulled down or built yourself
and you can use the `docker images` command to see the images
available to you locally.

    # Usage: [sudo] docker images
    # Example:
    $ docker images

    REPOSITORY          TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
    myUserName/nginx    latest              a0d6c70867d2        41 seconds ago      578.8 MB
    nginx               latest              173c2dd28ab2        3 minutes ago       578.8 MB
    dockerfile/nginx    latest              0ade68db1d05        3 weeks ago         578.8 MB

### Building our own images

You can build your own images using a `Dockerfile` and the `docker
build` command. The `Dockerfile` is very flexible and provides a
powerful set of instructions for building applications into Docker
images. To learn more about the `Dockerfile` see the [`Dockerfile`
Reference](/reference/builder/) and [tutorial](https://www.docker.io/learn/dockerfile/).

## Working with containers

### Docker Containers

Docker containers run your applications and are built from Docker
images. In order to create or start a container, you need an image. This
could be the base `ubuntu` image or an image built and shared with you
or an image you've built yourself.

### Running a new container from an image

The easiest way to create a new container is to *run* one from an image
using the `docker run` command.

    # Usage: [sudo] docker run [arguments] ..
    # Example:
    $ docker run -d --name nginx_web nginx /usr/sbin/nginx
    25137497b2749e226dd08f84a17e4b2be114ddf4ada04125f130ebfe0f1a03d3

This will create a new container from an image called `nginx` which will
launch the command `/usr/sbin/nginx` when the container is run. We've
also given our container a name, `nginx_web`. When the container is run
Docker will return a container ID, a long string that uniquely
identifies our container. We use can the container's name or its string
to work with it.

Containers can be run in two modes:

* Interactive;
* Daemonized;

An interactive container runs in the foreground and you can connect to
it and interact with it, for example sign into a shell on that
container. A daemonized container runs in the background.

A container will run as long as the process you have launched inside it
is running, for example if the `/usr/bin/nginx` process stops running
the container will also stop.

### Listing containers

We can see a list of all the containers on our host using the `docker
ps` command. By default the `docker ps` command only shows running
containers. But we can also add the `-a` flag to show *all* containers:
both running and stopped.

    # Usage: [sudo] docker ps [-a]
    # Example:
    $ docker ps

    CONTAINER ID        IMAGE                     COMMAND             CREATED             STATUS              PORTS                NAMES
    842a50a13032        $ dockerfile/nginx:latest   nginx               35 minutes ago      Up 30 minutes       0.0.0.0:80->80/tcp   nginx_web

### Stopping a container

You can use the `docker stop` command to stop an active container. This
will gracefully end the active process.

    # Usage: [sudo] docker stop [container ID]
    # Example:
    $ docker stop nginx_web
    nginx_web

If the `docker stop` command succeeds it will return the name of
the container it has stopped.

> **Note:** 
> If you want you to more aggressively stop a container you can use the
> `docker kill` command.

### Starting a Container

Stopped containers can be started again.

    # Usage: [sudo] docker start [container ID]
    # Example:
    $ docker start nginx_web
    nginx_web

If the `docker start` command succeeds it will return the name of the
freshly started container.

## Next steps

Here we've learned the basics of how to interact with Docker images and
how to run and work with our first container.

### Understanding Docker

Visit [Understanding Docker](understanding-docker.md).

### Installing Docker

Visit the [installation](/installation/#installation) section.

### Get the whole story

[https://www.docker.io/the_whole_story/](https://www.docker.io/the_whole_story/)

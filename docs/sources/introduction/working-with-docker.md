page_title: Working with Docker and the Dockerfile
page_description: Working with Docker and The Dockerfile explained in depth
page_keywords: docker, introduction, documentation, about, technology, understanding, Dockerfile

# Working with Docker and the Dockerfile

*How to use and work with Docker?*

> **Warning! Don't let this long page bore you.**
> If you prefer a summary and would like to see how a specific command
> works, check out the glossary of all available client
> commands on our [User's Manual: Commands Reference](
> http://docs.docker.io/en/latest/reference/commandline/cli).

## Introduction

On the last page, [Understanding the Technology](technology.md), we covered the
components that make up Docker and learnt about the
underlying technology and *how* everything works.

Now, it is time to get practical and see *how to work with* the Docker client,
Docker containers and images and the `Dockerfile`.

> **Note:** You are encouraged to take a good look at the container,
> image and `Dockerfile` explanations here to have a better understanding
> on what exactly they are and to get an overall idea on how to work with
> them. On the next page (i.e., [Get Docker](get-docker.md)), you will be
> able to find links for platform-centric installation instructions.

## Elements of Docker

As we mentioned on the, [Understanding the Technology](technology.md) page, the main
elements of Docker are:

 - Containers;
 - Images, and;
 - The `Dockerfile`.

> **Note:** This page is more *practical* than *technical*. If you are
> interested in understanding how these tools work behind the scenes
> and do their job, you can always read more on
> [Understanding the Technology](technology.md).

## Working with the Docker client

In order to work with the Docker client, you need to have a host with
the Docker daemon installed and running.

### How to use the client

The client provides you a command-line interface to Docker. It is
accessed by running the `docker` binary.

> **Tip:** The below instructions can be considered a summary of our
> *interactive tutorial*. If you prefer a more hands-on approach without
> installing anything, why not give that a shot and check out the
> [Docker Interactive Tutorial](http://www.docker.io/interactivetutorial).

The `docker` client usage consists of passing a chain of arguments:

    # Usage:  [sudo] docker [option] [command] [arguments] ..
    # Example:
    docker run -i -t ubuntu /bin/bash

### Our first Docker command

Let's get started with our first Docker command by checking the
version of the currently installed Docker client using the `docker
version` command.

    # Usage: [sudo] docker version
    # Example:
    docker version

This command will not only provide you the version of Docker client you
are using, but also the version of Go (the programming language powering
Docker).

    Client version: 0.8.0
    Go version (client): go1.2

    Git commit (client): cc3a8c8
    Server version: 0.8.0

    Git commit (server): cc3a8c8
    Go version (server): go1.2

    Last stable version: 0.8.0

### Finding out all available commands

The user-centric nature of Docker means providing you a constant stream
of helpful instructions. This begins with the client itself.

In order to get a full list of available commands run the `docker`
binary:

    # Usage: [sudo] docker
    # Example:
    docker

You will get an output with all currently available commands.

    Commands:
         attach    Attach to a running container
         build     Build a container from a Dockerfile
         commit    Create a new image from a container's changes
    . . .

### Command usage instructions

The same way used to learn all available commands can be repeated to find
out usage instructions for a specific command.

Try typing Docker followed with a `[command]` to see the instructions:

    # Usage: [sudo] docker [command] [--help]
    # Example:
    docker attach
    Help outputs . . .

Or you can pass the `--help` flag to the `docker` binary.

    docker images --help

You will get an output with all available options:

    Usage: docker attach [OPTIONS] CONTAINER

    Attach to a running container

      --no-stdin=false: Do not attach stdin
      --sig-proxy=true: Proxify all received signal to the process (even in non-tty mode)

## Working with images

### Docker Images

As we've discovered a Docker image is a read-only template that we build
containers from. Every Docker container is launched from an image and
you can use both images provided by others, for example we've discovered
the base `ubuntu` image provided by Docker, as well as images built by
others. For example we can build an image that runs Apache and our own
web application as a starting point to launch containers.

### Searching for images

To search for Docker image we use the `docker search` command. The
`docker search` command returns a list of all images that match your
search criteria together with additional, useful information about that
image. This includes information such as social metrics like how many
other people like the image - we call these "likes" *stars*. We also
tell you if an image is *trusted*. A *trusted* image is built from a
known source and allows you to introspect in greater detail how the
image is constructed.

    # Usage: [sudo] docker search [image name]
    # Example:
    docker search nginx

    NAME                                     DESCRIPTION                                     STARS     OFFICIAL   TRUSTED
    dockerfile/nginx                         Trusted Nginx (http://nginx.org/) Build         6                    [OK]
    paintedfox/nginx-php5                    A docker image for running Nginx with PHP5.     3                    [OK]
    dockerfiles/django-uwsgi-nginx           Dockerfile and configuration files to buil...   2                    [OK]
    . . .

> **Note:** To learn more about trusted builds, check out [this]
(http://blog.docker.io/2013/11/introducing-trusted-builds) blog post.

### Downloading an image

Downloading a Docker image is called *pulling*. To do this we hence use the
`docker pull` command.

    # Usage: [sudo] docker pull [image name]
    # Example:
    docker pull dockerfile/nginx

    Pulling repository dockerfile/nginx
    0ade68db1d05: Pulling dependent layers
    27cf78414709: Download complete
    b750fe79269d: Download complete
    . . .

As you can see, Docker will download, one by one, all the layers forming
the final image. This demonstrates the *building block* philosophy of
Docker.

### Listing available images

In order to get a full list of available images, you can use the
`docker images` command.

    # Usage: [sudo] docker images
    # Example:
    docker images

    REPOSITORY          TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
    myUserName/nginx    latest              a0d6c70867d2        41 seconds ago      578.8 MB
    nginx               latest              173c2dd28ab2        3 minutes ago       578.8 MB
    dockerfile/nginx    latest              0ade68db1d05        3 weeks ago         578.8 MB

## Working with containers

### Docker Containers

Docker containers are directories on your Docker host that are built
from Docker images. In order to create or start a container, you need an
image. This could be the base `ubuntu` image or an image built and
shared with you or an image you've built yourself.

### Running a new container from an image

The easiest way to create a new container is to *run* one from an image.

    # Usage: [sudo] docker run [arguments] ..
    # Example:
    docker run -d --name nginx_web nginx /usr/sbin/nginx

This will create a new container from an image called `nginx` which will
launch the command `/usr/sbin/nginx` when the container is run. We've
also given our container a name, `nginx_web`.

Containers can be run in two modes:

* Interactive;
* Daemonized;

An interactive container runs in the foreground and you can connect to
it and interact with it. A daemonized container runs in the background.

A container will run as long as the process you have launched inside it
is running, for example if the `/usr/bin/nginx` process stops running
the container will also stop.

### Listing containers

We can see a list of all the containers on our host using the `docker
ps` command. By default the `docker ps` commands only shows running
containers. But we can also add the `-a` flag to show *all* containers -
both running and stopped.

    # Usage: [sudo] docker ps [-a]
    # Example:
    docker ps

    CONTAINER ID        IMAGE                     COMMAND             CREATED             STATUS              PORTS                NAMES
    842a50a13032        dockerfile/nginx:latest   nginx               35 minutes ago      Up 30 minutes       0.0.0.0:80->80/tcp   nginx_web

### Stopping a container

You can use the `docker stop` command to stop an active container. This will gracefully
end the active process.

    # Usage: [sudo] docker stop [container ID]
    # Example:
    docker stop nginx_web
    nginx_web

If the `docker stop` command succeeds it will return the name of
the container it has stopped.

### Starting a Container

Stopped containers can be started again.

    # Usage: [sudo] docker start [container ID]
    # Example:
    docker start nginx_web
    nginx_web

If the `docker start` command succeeds it will return the name of the
freshly started container.

## Working with the Dockerfile

The `Dockerfile` holds the set of instructions Docker uses to build a Docker image.

> **Tip:** Below is a short summary of our full Dockerfile tutorial.  In
> order to get a better-grasp of how to work with these automation
> scripts, check out the [Dockerfile step-by-step
> tutorial](http://www.docker.io/learn/dockerfile).

A `Dockerfile` contains instructions written in the following format:

    # Usage: Instruction [arguments / command] ..
    # Example:
    FROM ubuntu

A `#` sign is used to provide a comment:

    # Comments ..

> **Tip:** The `Dockerfile` is very flexible and provides a powerful set
> of instructions for building applications. To learn more about the
> `Dockerfile` and it's instructions see the [Dockerfile
> Reference](http://docs.docker.io/en/latest/reference/builder).

### First steps with the Dockerfile

It's a good idea to add some comments to the start of your `Dockerfile`
to provide explanation and exposition to any future consumers, for
example:

    #
    # Dockerfile to install Nginx
    # VERSION 2 - EDITION 1

The first instruction in any `Dockerfile` must be the `FROM` instruction. The `FROM` instruction specifies the image name that this new image is built from, it is often a base image like `ubuntu`.

    # Base image used is Ubuntu:
    FROM ubuntu

Next, we recommend you use the `MAINTAINER` instruction to tell people who manages this image.

    # Maintainer: O.S. Tezer <ostezer at gmail com> (@ostezer)
    MAINTAINER O.S. Tezer, ostezer@gmail.com

After this we can add additional instructions that represent the steps
to build our actual image.

### Our Dockerfile so far

So far our `Dockerfile` will look like.

    # Dockerfile to install Nginx
    # VERSION 2 - EDITION 1
    FROM ubuntu
    MAINTAINER O.S. Tezer, ostezer@gmail.com

Let's install a package and configure an application inside our image. To do this we use a new
instruction: `RUN`. The `RUN` instruction executes commands inside our
image, for example. The instruction is just like running a command on
the command line inside a container.

    RUN echo "deb http://archive.ubuntu.com/ubuntu/ raring main universe" >> /etc/apt/sources.list
    RUN apt-get update
    RUN apt-get install -y nginx
    RUN echo "\ndaemon off;" >> /etc/nginx/nginx.conf

We can see here that we've *run* four instructions. Each time we run an
instruction a new layer is added to our image. Here's we've added an
Ubuntu package repository, updated the packages, installed the `nginx`
package and then echo'ed some configuration to the default
`/etc/nginx/nginx.conf` configuration file.

Let's specify another instruction, `CMD`, that tells Docker what command
to run when a container is created from this image.

    CMD /usr/sbin/nginx

We can now save this file and use it build an image.

### Using a Dockerfile

Docker uses the `Dockerfile` to build images. The build process is initiated by the `docker build` command.

    # Use the Dockerfile at the current location
    # Usage: [sudo] docker build .
    # Example:
    docker build -t="my_nginx_image" .

    Uploading context 25.09 kB
    Uploading context
    Step 0 : FROM ubuntu
      ---> 9cd978db300e
    Step 1 : MAINTAINER O.S. Tezer, ostezer@gmail.com
      ---> Using cache
      ---> 467542d0cdd3
    Step 2 : RUN echo "deb http://archive.ubuntu.com/ubuntu/ raring main universe" >> /etc/apt/sources.list
      ---> Using cache
      ---> 0a688bd2a48c
    Step 3 : RUN apt-get update
     ---> Running in de2937e8915a
    . . .
    Step 10 : CMD /usr/sbin/nginx
      ---> Running in b4908b9b9868
      ---> 626e92c5fab1
    Successfully built 626e92c5fab1

Here we can see that Docker has executed each instruction in turn and
each instruction has created a new layer in turn and each layer identified
by a new ID. The `-t` flag allows us to specify a name for our new
image, here `my_nginx_image`.

We can see our new image using the `docker images` command.

    docker images
    REPOSITORY          TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
    my_nginx_img        latest              626e92c5fab1        57 seconds ago      337.6 MB

## Where to go from here

### Understanding Docker

Visit [Understanding Docker](understanding-docker.md) in our Getting Started manual.

### Learn about parts of Docker and the underlying technology

Visit [Understanding the Technology](technology.md) in our Getting Started manual.

### Get the product and go hands-on

Visit [Get Docker](get-docker.md) in our Getting Started manual.

### Get the whole story

[https://www.docker.io/the_whole_story/](https://www.docker.io/the_whole_story/)

<!--[metadata]>
+++
aliases = ["/engine/userguide/dockerizing/"]
title = "Hello world in a container"
description = "A simple 'Hello world' exercise that introduced you to Docker."
keywords = ["docker guide, docker, docker platform, how to, dockerize, dockerizing apps, dockerizing applications, container,  containers"]
[menu.main]
parent="engine_learn"
weight=-6
+++
<![end-metadata]-->

# Hello world in a container

*So what's this Docker thing all about?*

Docker allows you to run applications, worlds you create, inside containers.
Running an application inside a container takes a single command: `docker run`.

>**Note**: Depending on your Docker system configuration, you may be required to
>preface each `docker` command on this page with `sudo`. To avoid this behavior,
>your system administrator can create a Unix group called `docker` and add users
>to it.

## Run a Hello world

Let's run a hello world container.

    $ docker run ubuntu /bin/echo 'Hello world'
    Hello world

You just launched your first container!

In this example:

* `docker run` runs a container.

* `ubuntu` is the image you run, for example the Ubuntu operating system image.
  When you specify an image, Docker looks first for the image on your
  Docker host. If the image does not exist locally, then the image is pulled from the public
  image registry [Docker Hub](https://hub.docker.com).

* `/bin/echo` is the command to run inside the new container.

The container launches. Docker creates a new Ubuntu
environment and executes the `/bin/echo` command inside it and then prints out:

    Hello world

So what happened to the container after that? Well, Docker containers
only run as long as the command you specify is active. Therefore, in the above example,
the container stops once the command is executed.

## Run an interactive container

Let's specify a new command to run in the container.

    $ docker run -t -i ubuntu /bin/bash
    root@af8bae53bdd3:/#

In this example:

* `docker run` runs a container.
* `ubuntu` is the image you would like to run.
* `-t` flag assigns a pseudo-tty or terminal inside the new container.
* `-i` flag allows you to make an interactive connection by
grabbing the standard in (`STDIN`) of the container.
* `/bin/bash` launches a Bash shell inside our container.

The container launches. We can see there is a
command prompt inside it:

    root@af8bae53bdd3:/#

Let's try running some commands inside the container:

    root@af8bae53bdd3:/# pwd
    /
    root@af8bae53bdd3:/# ls
    bin boot dev etc home lib lib64 media mnt opt proc root run sbin srv sys tmp usr var

In this example:

* `pwd` displays the current directory, the `/` root directory.  
* `ls` displays the directory listing of the root directory of a typical Linux file system.

Now, you can play around inside this container. When completed, run the `exit` command or enter Ctrl-D
to exit the interactive shell.

    root@af8bae53bdd3:/# exit

>**Note:** As with our previous container, once the Bash shell process has
finished, the container stops.

## Start a daemonized Hello world

Let's create a container that runs as a daemon.

    $ docker run -d ubuntu /bin/sh -c "while true; do echo hello world; sleep 1; done"
    1e5535038e285177d5214659a068137486f96ee5c2e85a4ac52dc83f2ebe4147

In this example:

* `docker run` runs the container.
* `-d` flag runs the container in the background (to daemonize it).
* `ubuntu` is the image you would like to run.

Finally, we specify a command to run:

    /bin/sh -c "while true; do echo hello world; sleep 1; done"


In the output, we do not see `hello world` but a long string:

    1e5535038e285177d5214659a068137486f96ee5c2e85a4ac52dc83f2ebe4147

This long string is called a *container ID*. It uniquely
identifies a container so we can work with it.

> **Note:**
> The container ID is a bit long and unwieldy. Later, we will cover the short
> ID and ways to name our containers to make
> working with them easier.

We can use this container ID to see what's happening with our `hello world` daemon.

First, let's make sure our container is running. Run the `docker ps` command.
The `docker ps` command queries the Docker daemon for information about all the containers it knows
about.

    $ docker ps
    CONTAINER ID  IMAGE         COMMAND               CREATED        STATUS       PORTS NAMES
    1e5535038e28  ubuntu  /bin/sh -c 'while tr  2 minutes ago  Up 1 minute        insane_babbage

In this example, we can see our daemonized container. The `docker ps` returns some useful
information:

* `1e5535038e28` is the shorter variant of the container ID.
* `ubuntu` is the used image.
* the command, status, and assigned name `insane_babbage`.


> **Note:**
> Docker automatically generates names for any containers started.
> We'll see how to specify your own names a bit later.

Now, we know the container is running. But is it doing what we asked it to do? To
see this we're going to look inside the container using the `docker logs`
command.

Let's use the container name `insane_babbage`.

    $ docker logs insane_babbage
    hello world
    hello world
    hello world
    . . .

In this example:

* `docker logs` looks inside the container and returns `hello world`.

Awesome! The daemon is working and you have just created your first
Dockerized application!

Next, run the `docker stop` command to stop our detached container.

    $ docker stop insane_babbage
    insane_babbage

The `docker stop` command tells Docker to politely stop the running
container and returns the name of the container it stopped.

Let's check it worked with the `docker ps` command.

    $ docker ps
    CONTAINER ID  IMAGE         COMMAND               CREATED        STATUS       PORTS NAMES

Excellent. Our container is stopped.

# Next steps

So far, you launched your first containers using the `docker run` command. You
ran an *interactive container* that ran in the foreground. You also ran a
*detached container* that ran in the background. In the process you learned
about several Docker commands:

* `docker ps` - Lists containers.
* `docker logs` - Shows us the standard output of a container.
* `docker stop` - Stops running containers.

Now, you have the basis learn more about Docker and how to do some more advanced
tasks. Go to ["*Run a simple application*"](usingdocker.md) to actually build a
web application with the Docker client.

page_title: Using Supervisor with Docker
page_description: How to use Supervisor process management with Docker
page_keywords: docker, supervisor, process management

# Using Supervisor with Docker

Note

-   This example assumes you have Docker running in daemon mode. For
    more information please see [*Check your Docker
    install*](../hello_world/#running-examples).
-   **If you don’t like sudo** then see [*Giving non-root
    access*](../../installation/binaries/#dockergroup)

Traditionally a Docker container runs a single process when it is
launched, for example an Apache daemon or a SSH server daemon. Often
though you want to run more than one process in a container. There are a
number of ways you can achieve this ranging from using a simple Bash
script as the value of your container’s `CMD`
instruction to installing a process management tool.

In this example we’re going to make use of the process management tool,
[Supervisor](http://supervisord.org/), to manage multiple processes in
our container. Using Supervisor allows us to better control, manage, and
restart the processes we want to run. To demonstrate this we’re going to
install and manage both an SSH daemon and an Apache daemon.

## Creating a Dockerfile

Let’s start by creating a basic `Dockerfile` for our
new image.

    FROM ubuntu:latest
    MAINTAINER examples@docker.io
    RUN echo "deb http://archive.ubuntu.com/ubuntu precise main universe" > /etc/apt/sources.list
    RUN apt-get update
    RUN apt-get upgrade -y

## Installing Supervisor

We can now install our SSH and Apache daemons as well as Supervisor in
our container.

    RUN apt-get install -y openssh-server apache2 supervisor
    RUN mkdir -p /var/run/sshd
    RUN mkdir -p /var/log/supervisor

Here we’re installing the `openssh-server`,
`apache2` and `supervisor`
(which provides the Supervisor daemon) packages. We’re also creating two
new directories that are needed to run our SSH daemon and Supervisor.

## Adding Supervisor’s configuration file

Now let’s add a configuration file for Supervisor. The default file is
called `supervisord.conf` and is located in
`/etc/supervisor/conf.d/`.

    ADD supervisord.conf /etc/supervisor/conf.d/supervisord.conf

Let’s see what is inside our `supervisord.conf`
file.

    [supervisord]
    nodaemon=true

    [program:sshd]
    command=/usr/sbin/sshd -D

    [program:apache2]
    command=/bin/bash -c "source /etc/apache2/envvars && exec /usr/sbin/apache2 -DFOREGROUND"

The `supervisord.conf` configuration file contains
directives that configure Supervisor and the processes it manages. The
first block `[supervisord]` provides configuration
for Supervisor itself. We’re using one directive, `nodaemon`
which tells Supervisor to run interactively rather than
daemonize.

The next two blocks manage the services we wish to control. Each block
controls a separate process. The blocks contain a single directive,
`command`, which specifies what command to run to
start each process.

## Exposing ports and running Supervisor

Now let’s finish our `Dockerfile` by exposing some
required ports and specifying the `CMD` instruction
to start Supervisor when our container launches.

    EXPOSE 22 80
    CMD ["/usr/bin/supervisord"]

Here we’ve exposed ports 22 and 80 on the container and we’re running
the `/usr/bin/supervisord` binary when the container
launches.

## Building our container

We can now build our new container.

    sudo docker build -t <yourname>/supervisord .

## Running our Supervisor container

Once we’ve got a built image we can launch a container from it.

    sudo docker run -p 22 -p 80 -t -i <yourname>/supervisord
    2013-11-25 18:53:22,312 CRIT Supervisor running as root (no user in config file)
    2013-11-25 18:53:22,312 WARN Included extra file "/etc/supervisor/conf.d/supervisord.conf" during parsing
    2013-11-25 18:53:22,342 INFO supervisord started with pid 1
    2013-11-25 18:53:23,346 INFO spawned: 'sshd' with pid 6
    2013-11-25 18:53:23,349 INFO spawned: 'apache2' with pid 7
    . . .

We’ve launched a new container interactively using the
`docker run` command. That container has run
Supervisor and launched the SSH and Apache daemons with it. We’ve
specified the `-p` flag to expose ports 22 and 80.
From here we can now identify the exposed ports and connect to one or
both of the SSH and Apache daemons.

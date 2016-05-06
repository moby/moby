<!--[metadata]>
+++
aliases = ["/engine/articles/using_supervisord/"]
title = "Using Supervisor with Docker"
description = "How to use Supervisor process management with Docker"
keywords = ["docker, supervisor,  process management"]
[menu.main]
parent = "engine_admin"
+++
<![end-metadata]-->

# Using Supervisor with Docker

> **Note**:
> - **If you don't like sudo** then see [*Giving non-root
>   access*](../installation/binaries.md#giving-non-root-access)

Traditionally a Docker container runs a single process when it is launched, for
example an Apache daemon or a SSH server daemon. Often though you want to run
more than one process in a container. There are a number of ways you can
achieve this ranging from using a simple Bash script as the value of your
container's `CMD` instruction to installing a process management tool.

In this example you're going to make use of the process management tool,
[Supervisor](http://supervisord.org/), to manage multiple processes in a
container. Using Supervisor allows you to better control, manage, and restart
the processes inside the container. To demonstrate this we're going to install
and manage both an SSH daemon and an Apache daemon.

## Creating a Dockerfile

Let's start by creating a basic `Dockerfile` for our new image.

```Dockerfile
FROM ubuntu:16.04
MAINTAINER examples@docker.com
```

## Installing Supervisor

You can now install the SSH and Apache daemons as well as Supervisor in the
container.

```Dockerfile
RUN apt-get update && apt-get install -y openssh-server apache2 supervisor
RUN mkdir -p /var/lock/apache2 /var/run/apache2 /var/run/sshd /var/log/supervisor
```

The first `RUN` instruction installs the `openssh-server`, `apache2` and
`supervisor` (which provides the Supervisor daemon) packages. The next `RUN`
instruction creates four new directories that are needed to run the SSH daemon
and Supervisor.

## Adding Supervisor's configuration file

Now let's add a configuration file for Supervisor. The default file is called
`supervisord.conf` and is located in `/etc/supervisor/conf.d/`.

```Dockerfile
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf
```

Let's see what is inside the `supervisord.conf` file.

```ini
[supervisord]
nodaemon=true

[program:sshd]
command=/usr/sbin/sshd -D

[program:apache2]
command=/bin/bash -c "source /etc/apache2/envvars && exec /usr/sbin/apache2 -DFOREGROUND"
```

The `supervisord.conf` configuration file contains directives that configure
Supervisor and the processes it manages. The first block `[supervisord]`
provides configuration for Supervisor itself. The `nodaemon` directive is used,
which tells Supervisor to run interactively rather than daemonize.

The next two blocks manage the services we wish to control. Each block controls
a separate process. The blocks contain a single directive, `command`, which
specifies what command to run to start each process.

## Exposing ports and running Supervisor

Now let's finish the `Dockerfile` by exposing some required ports and
specifying the `CMD` instruction to start Supervisor when our container
launches.

```Dockerfile
EXPOSE 22 80
CMD ["/usr/bin/supervisord"]
```

These instructions tell Docker that ports 22 and 80 are exposed  by the
container and that the `/usr/bin/supervisord` binary should be executed when
the container launches.

## Building our image

Your completed Dockerfile now looks like this:

```Dockerfile
FROM ubuntu:16.04
MAINTAINER examples@docker.com

RUN apt-get update && apt-get install -y openssh-server apache2 supervisor
RUN mkdir -p /var/lock/apache2 /var/run/apache2 /var/run/sshd /var/log/supervisor

COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf

EXPOSE 22 80
CMD ["/usr/bin/supervisord"]
```

And your `supervisord.conf` file looks like this;

```ini
[supervisord]
nodaemon=true

[program:sshd]
command=/usr/sbin/sshd -D

[program:apache2]
command=/bin/bash -c "source /etc/apache2/envvars && exec /usr/sbin/apache2 -DFOREGROUND"
```


You can now build the image using this command;

```bash
$ docker build -t mysupervisord .
```

## Running your Supervisor container

Once you have built your image you can launch a container from it.

```bash
$ docker run -p 22 -p 80 -t -i mysupervisord
2013-11-25 18:53:22,312 CRIT Supervisor running as root (no user in config file)
2013-11-25 18:53:22,312 WARN Included extra file "/etc/supervisor/conf.d/supervisord.conf" during parsing
2013-11-25 18:53:22,342 INFO supervisord started with pid 1
2013-11-25 18:53:23,346 INFO spawned: 'sshd' with pid 6
2013-11-25 18:53:23,349 INFO spawned: 'apache2' with pid 7
...
```

You launched a new container interactively using the `docker run` command.
That container has run Supervisor and launched the SSH and Apache daemons with
it. We've specified the `-p` flag to expose ports 22 and 80. From here we can
now identify the exposed ports and connect to one or both of the SSH and Apache
daemons.

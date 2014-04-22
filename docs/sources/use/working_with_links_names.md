page_title: Link Containers
page_description: How to create and use both links and names
page_keywords: Examples, Usage, links, linking, docker, documentation, examples, names, name, container naming

# Link Containers

## Introduction

From version 0.6.5 you are now able to `name` a
container and `link` it to another container by
referring to its name. This will create a parent -\> child relationship
where the parent container can see selected information about its child.

## Container Naming

New in version v0.6.5.

You can now name your container by using the `--name`
flag. If no name is provided, Docker will automatically
generate a name. You can see this name using the `docker ps`
command.

    # format is "sudo docker run --name <container_name> <image_name> <command>"
    $ sudo docker run --name test ubuntu /bin/bash

    # the flag "-a" Show all containers. Only running containers are shown by default.
    $ sudo docker ps -a
    CONTAINER ID        IMAGE                            COMMAND             CREATED             STATUS              PORTS               NAMES
    2522602a0d99        ubuntu:12.04                     /bin/bash           14 seconds ago      Exit 0                                  test

## Links: service discovery for docker

New in version v0.6.5.

Links allow containers to discover and securely communicate with each
other by using the flag `-link name:alias`.
Inter-container communication can be disabled with the daemon flag
`-icc=false`. With this flag set to
`false`, Container A cannot access Container B
unless explicitly allowed via a link. This is a huge win for securing
your containers. When two containers are linked together Docker creates
a parent child relationship between the containers. The parent container
will be able to access information via environment variables of the
child such as name, exposed ports, IP and other selected environment
variables.

When linking two containers Docker will use the exposed ports of the
container to create a secure tunnel for the parent to access. If a
database container only exposes port 8080 then the linked container will
only be allowed to access port 8080 and nothing else if inter-container
communication is set to false.

For example, there is an image called `crosbymichael/redis`
that exposes the port 6379 and starts the Redis server. Let’s
name the container as `redis` based on that image
and run it as daemon.

    $ sudo docker run -d -name redis crosbymichael/redis

We can issue all the commands that you would expect using the name
`redis`; start, stop, attach, using the name for our
container. The name also allows us to link other containers into this
one.

Next, we can start a new web application that has a dependency on Redis
and apply a link to connect both containers. If you noticed when running
our Redis server we did not use the `-p` flag to
publish the Redis port to the host system. Redis exposed port 6379 and
this is all we need to establish a link.

    $ sudo docker run -t -i -link redis:db -name webapp ubuntu bash

When you specified `-link redis:db` you are telling
Docker to link the container named `redis` into this
new container with the alias `db`. Environment
variables are prefixed with the alias so that the parent container can
access network and environment information from the containers that are
linked into it.

If we inspect the environment variables of the second container, we
would see all the information about the child container.

    $ root@4c01db0b339c:/# env

    HOSTNAME=4c01db0b339c
    DB_NAME=/webapp/db
    TERM=xterm
    DB_PORT=tcp://172.17.0.8:6379
    DB_PORT_6379_TCP=tcp://172.17.0.8:6379
    DB_PORT_6379_TCP_PROTO=tcp
    DB_PORT_6379_TCP_ADDR=172.17.0.8
    DB_PORT_6379_TCP_PORT=6379
    PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
    PWD=/
    SHLVL=1
    HOME=/
    container=lxc
    _=/usr/bin/env
    root@4c01db0b339c:/#

Accessing the network information along with the environment of the
child container allows us to easily connect to the Redis service on the
specific IP and port in the environment.

> **Note**:
> These Environment variables are only set for the first process in the
> container. Similarly, some daemons (such as `sshd`)
> will scrub them when spawning shells for connection.

You can work around this by storing the initial `env`
in a file, or looking at `/proc/1/environ`.

Running `docker ps` shows the 2 containers, and the
`webapp/db` alias name for the Redis container.

    $ docker ps
    CONTAINER ID        IMAGE                        COMMAND                CREATED              STATUS              PORTS               NAMES
    4c01db0b339c        ubuntu:12.04                 bash                   17 seconds ago       Up 16 seconds                           webapp
    d7886598dbe2        crosbymichael/redis:latest   /redis-server --dir    33 minutes ago       Up 33 minutes       6379/tcp            redis,webapp/db

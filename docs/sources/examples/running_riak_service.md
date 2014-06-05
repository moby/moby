page_title: Dockerizing a Riak service
page_description: Build a Docker image with Riak pre-installed
page_keywords: docker, example, package installation, networking, riak

# Dockerizing a Riak Service

The goal of this example is to show you how to build a Docker image with
Riak pre-installed.

## Creating a Dockerfile

Create an empty file called `Dockerfile`:

    $ touch Dockerfile

Next, define the parent image you want to use to build your image on top
of. We'll use [Ubuntu](https://index.docker.io/_/ubuntu/) (tag:
`latest`), which is available on [Docker Hub](http://index.docker.io):

    # Riak
    #
    # VERSION       0.1.0

    # Use the Ubuntu base image provided by dotCloud
    FROM ubuntu:latest
    MAINTAINER Hector Castro hector@basho.com

Next, we update the APT cache and apply any updates:

    # Update the APT cache
    RUN sed -i.bak 's/main$/main universe/' /etc/apt/sources.list
    RUN apt-get update
    RUN apt-get upgrade -y

After that, we install and setup a few dependencies:

 - `curl` is used to download Basho's APT
    repository key
 - `lsb-release` helps us derive the Ubuntu release
    codename
 - `openssh-server` allows us to login to
    containers remotely and join Riak nodes to form a cluster
 - `supervisor` is used manage the OpenSSH and Riak
    processes

<!-- -->

    # Install and setup project dependencies
    RUN apt-get install -y curl lsb-release supervisor openssh-server

    RUN mkdir -p /var/run/sshd
    RUN mkdir -p /var/log/supervisor

    RUN locale-gen en_US en_US.UTF-8

    ADD supervisord.conf /etc/supervisor/conf.d/supervisord.conf

    RUN echo 'root:basho' | chpasswd

Next, we add Basho's APT repository:

    RUN curl -s http://apt.basho.com/gpg/basho.apt.key | apt-key add --
    RUN echo "deb http://apt.basho.com $(lsb_release -cs) main" > /etc/apt/sources.list.d/basho.list
    RUN apt-get update

After that, we install Riak and alter a few defaults:

    # Install Riak and prepare it to run
    RUN apt-get install -y riak
    RUN sed -i.bak 's/127.0.0.1/0.0.0.0/' /etc/riak/app.config
    RUN echo "ulimit -n 4096" >> /etc/default/riak

Almost there. Next, we add a hack to get us by the lack of
`initctl`:

    # Hack for initctl
    # See: https://github.com/dotcloud/docker/issues/1024
    RUN dpkg-divert --local --rename --add /sbin/initctl
    RUN ln -s /bin/true /sbin/initctl

Then, we expose the Riak Protocol Buffers and HTTP interfaces, along
with SSH:

    # Expose Riak Protocol Buffers and HTTP interfaces, along with SSH
    EXPOSE 8087 8098 22

Finally, run `supervisord` so that Riak and OpenSSH
are started:

    CMD ["/usr/bin/supervisord"]

## Create a supervisord configuration file

Create an empty file called `supervisord.conf`. Make
sure it's at the same directory level as your `Dockerfile`:

    touch supervisord.conf

Populate it with the following program definitions:

    [supervisord]
    nodaemon=true

    [program:sshd]
    command=/usr/sbin/sshd -D
    stdout_logfile=/var/log/supervisor/%(program_name)s.log
    stderr_logfile=/var/log/supervisor/%(program_name)s.log
    autorestart=true

    [program:riak]
    command=bash -c ". /etc/default/riak && /usr/sbin/riak console"
    pidfile=/var/log/riak/riak.pid
    stdout_logfile=/var/log/supervisor/%(program_name)s.log
    stderr_logfile=/var/log/supervisor/%(program_name)s.log

## Build the Docker image for Riak

Now you should be able to build a Docker image for Riak:

    $ docker build -t "<yourname>/riak" .

## Next steps

Riak is a distributed database. Many production deployments consist of
[at least five nodes](
http://basho.com/why-your-riak-cluster-should-have-at-least-five-nodes/).
See the [docker-riak](https://github.com/hectcastro/docker-riak) project
details on how to deploy a Riak cluster using Docker and Pipework.

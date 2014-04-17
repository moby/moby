page_title: Installation on Fedora
page_description: Please note this project is currently under heavy development. It should not be used in production.
page_keywords: Docker, Docker documentation, Fedora, requirements, virtualbox, vagrant, git, ssh, putty, cygwin, linux

# Fedora

Note

Docker is still under heavy development! We don’t recommend using it in
production yet, but we’re getting closer with each release. Please see
our blog post, ["Getting to Docker
1.0"](http://blog.docker.io/2013/08/getting-to-docker-1-0/)

Note

This is a community contributed installation path. The only ‘official’
installation is using the [*Ubuntu*](../ubuntulinux/#ubuntu-linux)
installation path. This version may be out of date because it depends on
some binaries to be updated and published

Docker is available in **Fedora 19 and later**. Please note that due to
the current Docker limitations Docker is able to run only on the **64
bit** architecture.

## Installation

The `docker-io` package provides Docker on Fedora.

If you have the (unrelated) `docker` package
installed already, it will conflict with `docker-io`
.literal}. There’s a [bug
report](https://bugzilla.redhat.com/show_bug.cgi?id=1043676) filed for
it. To proceed with `docker-io` installation on
Fedora 19, please remove `docker` first.

    sudo yum -y remove docker

For Fedora 20 and later, the `wmdocker` package will
provide the same functionality as `docker` and will
also not conflict with `docker-io`.

    sudo yum -y install wmdocker
    sudo yum -y remove docker

Install the `docker-io` package which will install
Docker on our host.

    sudo yum -y install docker-io

To update the `docker-io` package:

    sudo yum -y update docker-io

Now that it’s installed, let’s start the Docker daemon.

    sudo systemctl start docker

If we want Docker to start at boot, we should also:

    sudo systemctl enable docker

Now let’s verify that Docker is working.

    sudo docker run -i -t fedora /bin/bash

**Done!**, now continue with the [*Hello
World*](../../examples/hello_world/#hello-world) example.

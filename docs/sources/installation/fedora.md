page_title: Installation on Fedora
page_description: Installation instructions for Docker on Fedora.
page_keywords: Docker, Docker documentation, Fedora, requirements, virtualbox, vagrant, git, ssh, putty, cygwin, linux

# Fedora

Docker is available in **Fedora 19 and later**. Please note that due to
the current Docker limitations Docker is able to run only on the **64
bit** architecture.

## Installation

The `docker-io` package provides Docker on Fedora.

If you have the (unrelated) `docker` package installed already, it will
conflict with `docker-io`. There's a [bug
report](https://bugzilla.redhat.com/show_bug.cgi?id=1043676) filed for
it. To proceed with `docker-io` installation on Fedora 19, please remove
`docker` first.

    $ sudo yum -y remove docker

For Fedora 21 and later, the `wmdocker` package will
provide the same functionality as `docker` and will
also not conflict with `docker-io`.

    $ sudo yum -y install wmdocker
    $ sudo yum -y remove docker

Install the `docker-io` package which will install
Docker on our host.

    $ sudo yum -y install docker-io

To update the `docker-io` package:

    $ sudo yum -y update docker-io

Now that it's installed, let's start the Docker daemon.

    $ sudo systemctl start docker

If we want Docker to start at boot, we should also:

    $ sudo systemctl enable docker

Now let's verify that Docker is working.

    $ sudo docker run -i -t fedora /bin/bash

> Note: If you get a `Cannot start container` error mentioning SELinux
> or permission denied, you may need to update the SELinux policies.
> This can be done using `sudo yum upgrade selinux-policy` and then rebooting.

## Granting rights to users to use Docker

Fedora 19 and 20 shipped with Docker 0.11. The package has already been updated
to 1.0 in Fedora 20. If you are still using the 0.11 version you will need to
grant rights to users of Docker.

The `docker` command line tool contacts the `docker` daemon process via a
socket file `/var/run/docker.sock` owned by group `docker`. One must be 
member of that group in order to contact the `docker -d` process.

    $ usermod -a -G docker login_name

Adding users to the `docker` group is *not* necessary for Docker versions 1.0
and above.

## Custom daemon options

If you need to add an HTTP Proxy, set a different directory or partition for the
Docker runtime files, or make other customizations, read our systemd article to
learn how to [customize your systemd Docker daemon options](/articles/systemd/).

## What next?

Continue with the [User Guide](/userguide/).


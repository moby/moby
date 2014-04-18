page_title: Installation on openSUSE
page_description: Please note this project is currently under heavy development. It should not be used in production.
page_keywords: openSUSE, virtualbox, docker, documentation, installation

# openSUSE

> **Note**:
> Docker is still under heavy development! We don’t recommend using it in
> production yet, but we’re getting closer with each release. Please see
> our blog post, [Getting to Docker 1.0](
> http://blog.docker.io/2013/08/getting-to-docker-1-0/)

> **Note**:
> This is a community contributed installation path. The only ‘official’
> installation is using the [*Ubuntu*](../ubuntulinux/#ubuntu-linux)
> installation path. This version may be out of date because it depends on
> some binaries to be updated and published

Docker is available in **openSUSE 12.3 and later**. Please note that due
to the current Docker limitations Docker is able to run only on the **64
bit** architecture.

## Installation

The `docker` package from the [Virtualization
project](https://build.opensuse.org/project/show/Virtualization) on
[OBS](https://build.opensuse.org/) provides Docker on openSUSE.

To proceed with Docker installation please add the right Virtualization
repository.

    # openSUSE 12.3
    sudo zypper ar -f http://download.opensuse.org/repositories/Virtualization/openSUSE_12.3/ Virtualization

    # openSUSE 13.1
    sudo zypper ar -f http://download.opensuse.org/repositories/Virtualization/openSUSE_13.1/ Virtualization

Install the Docker package.

    sudo zypper in docker

It’s also possible to install Docker using openSUSE’s 1-click install.
Just visit [this](http://software.opensuse.org/package/docker) page,
select your openSUSE version and click on the installation link. This
will add the right repository to your system and it will also install
the docker package.

Now that it’s installed, let’s start the Docker daemon.

    sudo systemctl start docker

If we want Docker to start at boot, we should also:

    sudo systemctl enable docker

The docker package creates a new group named docker. Users, other than
root user, need to be part of this group in order to interact with the
Docker daemon.

    sudo usermod -G docker <username>

**Done!**, now continue with the [*Hello
World*](../../examples/hello_world/#hello-world) example.

page_title: Installation on Mac OS X
page_description: Instructions for installing Docker on OS X using boot2docker.
page_keywords: Docker, Docker documentation, requirements, boot2docker, VirtualBox, SSH, Linux, OSX, OS X, Mac

# Installing Docker on Mac OS X

> **Note**:
> Docker is still under heavy development! We don't recommend using it in
> production yet, but we're getting closer with each release. Please see
> our blog post, [Getting to Docker 1.0](
> http://blog.docker.io/2013/08/getting-to-docker-1-0/)

> **Note:**
> Docker is supported on Mac OS X 10.6 "Snow Leopard" or newer.

Docker has two key components: the Docker daemon and the `docker` binary
which acts as a client. The client passes instructions to the daemon
which builds, runs and manages your Docker containers. As Docker uses
some Linux-specific kernel features you can't use it directly on OS X.
Instead we run the Docker daemon inside a lightweight virtual machine on your local
OS X host. We can then use a native client `docker` binary to communicate
with the Docker daemon inside our virtual machine. To make this process
easier we've designed a helper application called
[boot2docker](https://github.com/boot2docker/boot2docker) to install
that virtual machine and run our Docker daemon.

[boot2docker](https://github.com/boot2docker/boot2docker) uses
VirtualBox to create the virtual machine so we'll need to install that
first.

## Installing VirtualBox

Docker on OS X needs VirtualBox to run. To begin with, head over to
[VirtualBox Download Page](https://www.virtualbox.org/wiki/Downloads)
and get the tool for `OS X hosts x86/amd64`.

Once the download is complete, open the disk image, run `VirtualBox.pkg`
and install VirtualBox.

> **Note**:
> Do not simply copy the package without running the
> installer.

## Installing boot2docker manually

### Downloading the boot2docker script

[boot2docker](https://github.com/boot2docker/boot2docker) provides a
handy script to manage the VM running the Docker daemon. It also takes
care of the installation of that VM.

Open up a new terminal window and run the following commands to get
boot2docker:

    # Enter the installation directory
    $ mkdir -p ~/bin
    $ cd ~/bin

    # Get the file
    $ curl https://raw.githubusercontent.com/boot2docker/boot2docker/master/boot2docker > boot2docker

    # Mark it executable
    $ chmod +x boot2docker

### Installing the Docker OS X Client

The Docker daemon is accessed using the `docker` binary.

Run the following commands to get it downloaded and set up:

    # Get the docker binary
    $ DIR=$(mktemp -d ${TMPDIR:-/tmp}/dockerdl.XXXXXXX) && \
      curl -f -o $DIR/ld.tgz https://get.docker.io/builds/Darwin/x86_64/docker-latest.tgz && \
      gunzip $DIR/ld.tgz && \
      tar xvf $DIR/ld.tar -C $DIR/ && \
      cp $DIR/usr/local/bin/docker ./docker

    # Copy the executable file
    $ sudo mkdir -p /usr/local/bin
    $ sudo cp docker /usr/local/bin/

### Configure the Docker OS X Client

The Docker client, `docker`, uses an environment variable `DOCKER_HOST`
to specify the location of the Docker daemon to connect to. Specify your
local boot2docker virtual machine as the value of that variable.

    $ export DOCKER_HOST=tcp://127.0.0.1:4243

## Installing boot2docker with Homebrew

If you are using Homebrew on your machine, simply run the following
command to install `boot2docker`:

    $ brew install boot2docker

Run the following command to install the Docker client:

    $ brew install docker

And that's it! Let's check out how to use it.

# How To Use Docker On Mac OS X

## Running the Docker daemon via boot2docker

Firstly we need to initialize our boot2docker virtual machine. Run the
`boot2docker` command.

    $ boot2docker init

This will setup our initial virtual machine.

Next we need to start the Docker daemon.

    $ boot2docker up

There are a variety of others commands available using the `boot2docker`
script. You can see these like so:

    $ boot2docker
    Usage ./boot2docker {init|start|up|pause|stop|restart|status|info|delete|ssh|download}

## The Docker client

Once the virtual machine with the Docker daemon is up, you can use the `docker`
binary just like any other application.

    $ docker version
    Client version: 0.10.0
    Client API version: 1.10
    Server version: 0.10.0
    Server API version: 1.10
    Last stable version: 0.10.0

## Using Docker port forwarding with boot2docker

In order to forward network ports from Docker with boot2docker we need to
manually forward the port range Docker uses inside VirtualBox. To do
this we take the port range that Docker uses by default with the `-P`
option, ports 49000-49900, and run the following command.

> **Note:**
> The boot2docker virtual machine must be powered off for this
> to work.

    for i in {49000..49900}; do
     VBoxManage modifyvm "boot2docker-vm" --natpf1 "tcp-port$i,tcp,,$i,,$i";
     VBoxManage modifyvm "boot2docker-vm" --natpf1 "udp-port$i,udp,,$i,,$i";
    done

## Connecting to the VM via SSH

If you feel the need to connect to the VM, you can simply run:

    $ boot2docker ssh

    # User: docker
    # Pwd:  tcuser

If SSH complains about keys then run:

    $ ssh-keygen -R '[localhost]:2022'

## Upgrading to a newer release of boot2docker

To upgrade an initialized boot2docker virtual machine, you can use the
following 3 commands. Your virtual machine's disk will not be changed,
so you won't lose your images and containers:

    $ boot2docker stop
    $ boot2docker download
    $ boot2docker start

# Learn More

## boot2docker

See the GitHub page for
[boot2docker](https://github.com/boot2docker/boot2docker).

# Next steps

You can now continue with the [*Hello
World*](/examples/hello_world/#hello-world) example.


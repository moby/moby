page_title: Other ways to install Docker on Mac OS X
page_description: Instructions for other methods of installing Docker on OS X using boot2docker.
page_keywords: Docker, Docker documentation, requirements, boot2docker, VirtualBox, SSH, Linux, OSX, OS X, Mac

# Other ways to install Docker on Mac OS X

## Installing manually

### Installing VirtualBox

Docker on OS X needs VirtualBox to run. To begin with, head over to
[VirtualBox Download Page](https://www.virtualbox.org/wiki/Downloads)
and get the tool for `OS X hosts x86/amd64`.

Once the download is complete, open the disk image, run `VirtualBox.pkg`
and install VirtualBox.

### Installing boot2docker

> **Note**:
> Do not simply copy the package without running the
> installer.

[boot2docker](http://boot2docker.io) provides a
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

And that's it! [Let's check out how to use it.](/installation/mac/#how-to-use-docker-on-mac-os-x)
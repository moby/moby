Docker
======

Setup instructions
==================

Supported hosts
---------------

Right now, the officially supported hosts are:
* Ubuntu 12.10 (quantal)

Hosts that might work with slight kernel modifications, but are not officially supported:
* Ubuntu 12.04 (precise)

Step by step host setup
-----------------------

1. Set up your host of choice on a physical / virtual machine
2. Assume root identity on your newly installed environment (`sudo -s`)
3. Type the following commands:

    apt-get update
    apt-get install lxc wget
    debootstrap --arch=amd64 quantal /var/lib/docker/images/ubuntu/
4. Download the latest version of the [docker binaries](https://dl.dropbox.com/u/20637798/docker.tar.gz) (`wget https://dl.dropbox.com/u/20637798/docker.tar.gz`)
5. Extract the contents of the tar file `tar -xf docker.tar.gz`
6. Launch the docker daemon `./dockerd`


Client installation
-------------------

4. Download the latest version of the [docker binaries](https://dl.dropbox.com/u/20637798/docker.tar.gz) (`wget https://dl.dropbox.com/u/20637798/docker.tar.gz`)
5. Extract the contents of the tar file `tar -xf docker.tar.gz`
6. You can now use the docker client binary `./docker`. Consider adding it to your `PATH` for simplicity.

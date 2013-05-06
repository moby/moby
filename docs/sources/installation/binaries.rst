.. _binaries:

Binaries
========

  **Please note this project is currently under heavy development. It should not be used in production.**


Right now, the officially supported distributions are:

- Ubuntu 12.04 (precise LTS) (64-bit)
- Ubuntu 12.10 (quantal) (64-bit)


Install dependencies:
---------------------

::

    sudo apt-get install lxc bsdtar
    sudo apt-get install linux-image-extra-`uname -r`

The linux-image-extra package is needed on standard Ubuntu EC2 AMIs in order to install the aufs kernel module.

Install the docker binary:

::

    wget http://get.docker.io/builds/Linux/x86_64/docker-latest.tgz
    tar -xf docker-latest.tgz
    sudo cp ./docker-latest/docker /usr/local/bin

Note: docker currently only supports 64-bit Linux hosts.


Run the docker daemon
---------------------

::

    sudo docker -d &


Run your first container!
-------------------------

::

    docker run -i -t ubuntu /bin/bash



Continue with the :ref:`hello_world` example.

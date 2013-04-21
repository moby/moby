.. _ubuntu_linux:

Ubuntu Linux
============

  **Please note this project is currently under heavy development. It should not be used in production.**



Installing on Ubuntu 12.04 and 12.10

Right now, the officially supported distributions are:

* Ubuntu 12.04 (precise LTS)
* Ubuntu 12.10 (quantal)

Install dependencies:
---------------------

::

    sudo apt-get install lxc bsdtar
    sudo apt-get install linux-image-extra-`uname -r`

The linux-image-extra package is needed on standard Ubuntu EC2 AMIs in order to install the aufs kernel module.

Install the docker binary
-------------------------

::

    wget http://get.docker.io/builds/Linux/x86_64/docker-master.tgz
    tar -xf docker-master.tgz
    sudo cp ./docker-master /usr/local/bin

Note: docker currently only supports 64-bit Linux hosts.


Run the docker daemon
---------------------

::

    sudo docker -d &

Run your first container!
-------------------------

::
    docker run -i -t ubuntu /bin/bash


Check out more examples
-----------------------

Continue with the :ref:`hello_world` example.
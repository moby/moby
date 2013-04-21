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

    sudo apt-get install lxc wget bsdtar curl
    sudo apt-get install linux-image-extra-`uname -r`

The linux-image-extra package is needed on standard Ubuntu EC2 AMIs in order to install the aufs kernel module.

Install the latest docker binary:

::

    wget http://get.docker.io/builds/$(uname -s)/$(uname -m)/docker-master.tgz
    tar -xf docker-master.tgz

Run your first container!

::

    cd docker-master

::

    sudo ./docker run -i -t base /bin/bash


To run docker as a daemon, in the background, and allow non-root users to run ``docker`` start
docker -d

::

    sudo ./docker -d &


Consider adding docker to your PATH for simplicity.

Continue with the :ref:`hello_world` example.
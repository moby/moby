:title: Installation from Binaries
:description: This instruction set is meant for hackers who want to try out Docker on a variety of environments.
:keywords: binaries, installation, docker, documentation, linux

.. _binaries:

Binaries
========

.. include:: install_header.inc

**This instruction set is meant for hackers who want to try out Docker
on a variety of environments.**

Before following these directions, you should really check if a packaged version
of Docker is already available for your distribution.  We have packages for many
distributions, and more keep showing up all the time!


Check runtime dependencies
--------------------------

To run properly, docker needs the following software to be installed at runtime:

- GNU Tar version 1.26 or later
- iproute2 version 3.5 or later (build after 2012-05-21), and specifically the "ip" utility
- iptables version 1.4 or later
- The LXC utility scripts (http://lxc.sourceforge.net) version 0.8 or later
- Git version 1.7 or later
- XZ Utils 4.9 or later


Check kernel dependencies
-------------------------

Docker in daemon mode has specific kernel requirements. For details, see
http://docs.docker.io/en/latest/articles/kernel/

Note that Docker also has a client mode, which can run on virtually any linux kernel (it even builds
on OSX!).


Get the docker binary:
----------------------

.. code-block:: bash

    wget https://get.docker.io/builds/Linux/x86_64/docker-latest -O docker
    chmod +x docker


Run the docker daemon
---------------------

.. code-block:: bash

    # start the docker in daemon mode from the directory you unpacked
    sudo ./docker -d &

Upgrades
--------

To upgrade your manual installation of Docker, first kill the docker daemon:

.. code-block:: bash

   killall docker

Then follow the regular installation steps.


Run your first container!
-------------------------

.. code-block:: bash

    # check your docker version
    sudo ./docker version

    # run a container and open an interactive shell in the container
    sudo ./docker run -i -t ubuntu /bin/bash



Continue with the :ref:`hello_world` example.

:title: Installation from Binaries
:description: This instruction set is meant for hackers who want to try out Docker on a variety of environments.
:keywords: binaries, installation, docker, documentation, linux

.. _binaries:

Binaries
========

.. include:: install_header.inc

**This instruction set is meant for hackers who want to try out Docker
on a variety of environments.**

Right now, the officially supported distributions are:

- :ref:`ubuntu_precise`
- :ref:`ubuntu_raring`


But we know people have had success running it under

- Debian
- Suse
- :ref:`arch_linux`

Check Your Kernel
-----------------

Your host's Linux kernel must meet the Docker :ref:`kernel`

Get the docker binary:
----------------------

.. code-block:: bash

    wget --output-document=docker https://get.docker.io/builds/Linux/x86_64/docker-latest
    chmod +x docker


Run the docker daemon
---------------------

.. code-block:: bash

    # start the docker in daemon mode from the directory you unpacked
    sudo ./docker -d &


Run your first container!
-------------------------

.. code-block:: bash

    # check your docker version
    sudo ./docker version

    # run a container and open an interactive shell in the container
    sudo ./docker run -i -t ubuntu /bin/bash



Continue with the :ref:`hello_world` example.

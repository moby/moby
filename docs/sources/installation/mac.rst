:title: Installation on Mac OS X 10.6 Snow Leopard
:description: Please note this project is currently under heavy development. It should not be used in production.
:keywords: Docker, Docker documentation, requirements, virtualbox, ssh, linux, os x, osx, mac

.. _macosx:

========
Mac OS X
========

.. note::

   These instructions are available with the new release of Docker
   (version 0.8). However, they are subject to change.

.. include:: install_header.inc

Docker is supported on Mac OS X 10.6 "Snow Leopard" or newer.

How To Install Docker On Mac OS X
=================================

VirtualBox
----------

Docker on OS X needs VirtualBox to run. To begin with, head over to
`VirtualBox Download Page`_ and get the tool for ``OS X hosts x86/amd64``.

.. _VirtualBox Download Page: https://www.virtualbox.org/wiki/Downloads

Once the download is complete, open the disk image, run the set up file
(i.e. ``VirtualBox.pkg``) and install VirtualBox. Do not simply copy the
package without running the installer.

boot2docker
-----------

`boot2docker`_ provides a handy script to easily manage the VM running the
``docker`` daemon. It also takes care of the installation for the OS image
that is used for the job.

.. _GitHub page: https://github.com/boot2docker/boot2docker

With Homebrew
~~~~~~~~~~~~~

If you are using Homebrew on your machine, simply run the following command to install ``boot2docker``:

.. code-block:: bash

    brew install boot2docker

Manual installation
~~~~~~~~~~~~~~~~~~~

Open up a new terminal window, if you have not already.

Run the following commands to get boot2docker:

.. code-block:: bash

    # Enter the installation directory
    cd ~/bin

    # Get the file
    curl https://raw.github.com/boot2docker/boot2docker/master/boot2docker > boot2docker

    # Mark it executable
    chmod +x boot2docker

Docker OS X Client
------------------

The ``docker`` daemon is accessed using the ``docker`` client.

With Homebrew
~~~~~~~~~~~~~

Run the following command to install the ``docker`` client:

.. code-block:: bash

    brew install docker
    
Manual installation
~~~~~~~~~~~~~~~~~~~

Run the following commands to get it downloaded and set up:

.. code-block:: bash

    # Get the docker client file
    DIR=$(mktemp -d ${TMPDIR:-/tmp}/dockerdl.XXXXXXX) && \
    curl -f -o $DIR/ld.tgz https://get.docker.io/builds/Darwin/x86_64/docker-latest.tgz && \
    gunzip $DIR/ld.tgz && \
    tar xvf $DIR/ld.tar -C $DIR/ && \
    cp $DIR/usr/local/bin/docker ./docker

    # Set the environment variable for the docker daemon
    export DOCKER_HOST=tcp://127.0.0.1:4243

    # Copy the executable file
    sudo cp docker /usr/local/bin/

And that’s it! Let’s check out how to use it.

How To Use Docker On Mac OS X
=============================

The ``docker`` daemon (via boot2docker)
---------------------------------------

Inside the ``~/bin`` directory, run the following commands:

.. code-block:: bash

    # Initiate the VM
    ./boot2docker init

    # Run the VM (the docker daemon)
    ./boot2docker up

    # To see all available commands:
    ./boot2docker

    # Usage ./boot2docker {init|start|up|pause|stop|restart|status|info|delete|ssh|download}

The ``docker`` client
---------------------

Once the VM with the ``docker`` daemon is up, you can use the ``docker``
client just like any other application.

.. code-block:: bash

    docker version
    # Client version: 0.7.6
    # Go version (client): go1.2
    # Git commit (client): bc3b2ec
    # Server version: 0.7.5
    # Git commit (server): c348c04
    # Go version (server): go1.2

Forwarding VM Port Range to Host
--------------------------------

If we take the port range that docker uses by default with the -P option
(49000-49900), and forward same range from host to vm, we'll be able to interact
with our containers as if they were running locally:

.. code-block:: bash

   # vm must be powered off
   for i in {49000..49900}; do
    VBoxManage modifyvm "boot2docker-vm" --natpf1 "tcp-port$i,tcp,,$i,,$i";
    VBoxManage modifyvm "boot2docker-vm" --natpf1 "udp-port$i,udp,,$i,,$i";
   done

SSH-ing The VM
--------------

If you feel the need to connect to the VM, you can simply run:

.. code-block:: bash

    ./boot2docker ssh

    # User: docker
    # Pwd:  tcuser

You can now continue with the :ref:`hello_world` example.

Learn More
==========

boot2docker:
------------

See the GitHub page for `boot2docker`_.

.. _boot2docker: https://github.com/boot2docker/boot2docker

If SSH complains about keys:
----------------------------

.. code-block:: bash

    ssh-keygen -R '[localhost]:2022'

Upgrading to a newer release of boot2docker
-------------------------------------------

To upgrade an initialised VM, you can use the following 3 commands. Your persistence
disk will not be changed, so you won't lose your images and containers:

.. code-block:: bash

    ./boot2docker stop
    ./boot2docker download
    ./boot2docker start

About the way Docker works on Mac OS X:
---------------------------------------

Docker has two key components: the ``docker`` daemon and the ``docker``
client. The tool works by client commanding the daemon. In order to
work and do its magic, the daemon makes use of some Linux Kernel
features (e.g. LXC, name spaces etc.), which are not supported by OS X.
Therefore, the solution of getting Docker to run on OS X consists of
running it inside a lightweight virtual machine. In order to simplify
things, Docker comes with a bash script to make this whole process as
easy as possible (i.e. boot2docker).

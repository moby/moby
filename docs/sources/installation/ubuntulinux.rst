.. _ubuntu_linux:

Ubuntu Linux
============

  **Please note this project is currently under heavy development. It should not be used in production.**


Right now, the officially supported distributions are:

- Ubuntu 12.04 (precise LTS) (64-bit)
- Ubuntu 12.10 (quantal) (64-bit)

Dependencies
------------

The linux-image-extra package is only needed on standard Ubuntu EC2 AMIs in order to install the aufs kernel module.

.. code-block:: bash

   sudo apt-get install linux-image-extra-`uname -r` lxc bsdtar


Installation
------------

Docker is available as a Ubuntu PPA (Personal Package Archive),
`hosted on launchpad  <https://launchpad.net/~dotcloud/+archive/lxc-docker>`_
which makes installing Docker on Ubuntu very easy.



Add the custom package sources to your apt sources list. Copy and paste the following lines at once.

.. code-block:: bash

   sudo sh -c "echo 'deb http://ppa.launchpad.net/dotcloud/lxc-docker/ubuntu precise main' >> /etc/apt/sources.list"


Update your sources. You will see a warning that GPG signatures cannot be verified.

.. code-block:: bash

   sudo apt-get update


Now install it, you will see another warning that the package cannot be authenticated. Confirm install.

.. code-block:: bash

    curl -s http://get.docker.io/builds/$(uname -s)/$(uname -m)/docker-master.tgz |  tar  -zxf- docker-master/docker
    sudo cp docker-master/docker /usr/local/bin/docker


Verify it worked

.. code-block:: bash

   docker


**Done!**, now continue with the :ref:`hello_world` example.

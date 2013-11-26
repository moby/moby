:title: Requirements and Installation on Fedora
:description: Please note this project is currently under heavy development. It should not be used in production.
:keywords: Docker, Docker documentation, requirements, virtualbox, vagrant, git, ssh, putty, cygwin, linux

.. _fedora:

Fedora
======

.. include:: install_header.inc

.. include:: install_unofficial.inc

Docker is available in **Fedora 19 and later**. Please note that due to the
current Docker limitations Docker is able to run only on the **64 bit**
architecture.

Installation
------------

Firstly, let's make sure our Fedora host is up-to-date.

.. code-block:: bash

    sudo yum -y upgrade

Next let's install the ``docker-io`` package which will install Docker on our host.

.. code-block:: bash

   sudo yum -y install docker-io

Now it's installed lets start the Docker daemon.

.. code-block:: bash

    sudo systemctl start docker

If we want Docker to start at boot we should also:

.. code-block:: bash

   sudo systemctl enable docker

Now let's verify that Docker is working.

.. code-block:: bash

   sudo docker run -i -t ubuntu /bin/bash

**Done!**, now continue with the :ref:`hello_world` example.


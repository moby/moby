:title: Requirements and Installation on Fedora
:description: Please note this project is currently under heavy development. It should not be used in production.
:keywords: Docker, Docker documentation, fedora, requirements, virtualbox, vagrant, git, ssh, putty, cygwin, linux

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

The ``docker-io`` package provides Docker on fedora.

If you already have the (unrelated) ``docker`` package installed, it will
conflict with ``docker-io``. There's a `bug report`_ filed for it.
To proceed with ``docker-io`` installation on fedora 19, please remove
``docker`` first.

.. code-block:: bash

   sudo yum -y remove docker

For fedora 20 and above, the ``wmdocker`` package will provide the same
functionality as ``docker`` and will also not conflict with ``docker-io``

.. code-block:: bash

   sudo yum -y install wmdocker
   sudo yum -y remove docker

Install the ``docker-io`` package which will install Docker on our host.

.. code-block:: bash

   sudo yum -y install docker-io


To update the ``docker-io`` package

.. code-block:: bash

   sudo yum -y update docker-io

Now that it's installed, let's start the Docker daemon.

.. code-block:: bash

    sudo systemctl start docker

If we want Docker to start at boot, we should also:

.. code-block:: bash

   sudo systemctl enable docker

Now let's verify that Docker is working.

.. code-block:: bash

   sudo docker run -i -t mattdm/fedora /bin/bash

**Done!**, now continue with the :ref:`hello_world` example.

.. _bug report: https://bugzilla.redhat.com/show_bug.cgi?id=1043676

:title: Installation on openSUSE
:description: Docker installation on openSUSE.
:keywords: openSUSE, virtualbox, docker, documentation, installation

.. _openSUSE:

openSUSE
========

.. include:: install_header.inc

.. include:: install_unofficial.inc

Docker is available in **openSUSE 12.3 and later**. Please note that due to the
current Docker limitations Docker is able to run only on the **64 bit**
architecture.

Installation
------------

The ``docker`` package from the `Virtualization project`_ on `OBS`_ provides
Docker on openSUSE.


To proceed with Docker installation please add the right Virtualization
repository.

.. code-block:: bash

   # openSUSE 12.3
   sudo zypper ar -f http://download.opensuse.org/repositories/Virtualization/openSUSE_12.3/ Virtualization

   # openSUSE 13.1
   sudo zypper ar -f http://download.opensuse.org/repositories/Virtualization/openSUSE_13.1/ Virtualization


Install the Docker package.

.. code-block:: bash

   sudo zypper in docker

It's also possible to install Docker using openSUSE's 1-click install. Just
visit `this`_ page, select your openSUSE version and click on the installation
link. This will add the right repository to your system and it will
also install the `docker` package.

Now that it's installed, let's start the Docker daemon.

.. code-block:: bash

    sudo systemctl start docker

If we want Docker to start at boot, we should also:

.. code-block:: bash

   sudo systemctl enable docker

The `docker` package creates a new group named `docker`. Users, other than
`root` user, need to be part of this group in order to interact with the
Docker daemon.

.. code-block:: bash

   sudo usermod -G docker <username>


**Done!**, now continue with the :ref:`hello_world` example.

.. _Virtualization project: https://build.opensuse.org/project/show/Virtualization
.. _OBS: https://build.opensuse.org/
.. _this: http://software.opensuse.org/package/docker

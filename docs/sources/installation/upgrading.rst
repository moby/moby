:title: Upgrading
:description: These instructions are for upgrading Docker
:keywords: Docker, Docker documentation, upgrading docker, upgrade

.. _upgrading:

Upgrading
=========

The technique for upgrading ``docker`` to a newer version depends on
how you installed ``docker``.

.. versionadded:: 0.5.3
   You may wish to add a ``docker`` group to your system to avoid using sudo with ``docker``. (see :ref:`dockergroup`)


After ``apt-get``
-----------------

If you installed Docker using ``apt-get`` or Vagrant, then you should
use ``apt-get`` to upgrade.

.. versionadded:: 0.6
   Add Docker repository information to your system first.

.. code-block:: bash

   # Add the Docker repository key to your local keychain
   sudo sh -c "curl https://get.docker.io/gpg | apt-key add -"

   # Add the Docker repository to your apt sources list.
   sudo sh -c "echo deb https://get.docker.io/ubuntu docker main > /etc/apt/sources.list.d/docker.list"

   # update your sources list
   sudo apt-get update

   # install the latest
   sudo apt-get install lxc-docker


After manual installation
-------------------------

If you installed the Docker :ref:`binaries` then follow these steps:


.. code-block:: bash

   # kill the running docker daemon
   killall docker


.. code-block:: bash

   # get the latest binary
   wget http://get.docker.io/builds/Linux/x86_64/docker-latest -O docker
   
   # make it executable
   chmod +x docker


Start docker in daemon mode (``-d``) and disconnect, running the
daemon in the background (``&``). Starting as ``./docker`` guarantees
to run the version in your current directory rather than a version
which might reside in your path.

.. code-block:: bash

   # start the new version
   sudo ./docker -d &


Alternatively you can replace the docker binary in ``/usr/local/bin``.

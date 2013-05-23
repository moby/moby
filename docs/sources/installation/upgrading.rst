:title: Upgrading
:description: These instructions are for upgrading Docker
:keywords: Docker, Docker documentation, upgrading docker, upgrade

.. _upgrading:

Upgrading
============

**These instructions are for upgrading Docker**


After normal installation
-------------------------

If you installed Docker normally using apt-get or used Vagrant, use apt-get to upgrade.

.. code-block:: bash

   # update your sources list
   sudo apt-get update

   # install the latest
   sudo apt-get install lxc-docker


After manual installation
-------------------------

If you installed the Docker binary


.. code-block:: bash

   # kill the running docker daemon
   killall docker


.. code-block:: bash

   # get the latest binary
   wget http://get.docker.io/builds/Linux/x86_64/docker-latest.tgz


.. code-block:: bash

   # Unpack it to your current dir
   tar -xf docker-latest.tgz


Start docker in daemon mode (-d) and disconnect (&) starting ./docker will start the version in your current dir rather than a version which
might reside in your path.

.. code-block:: bash

   # start the new version
   sudo ./docker -d &


Alternatively you can replace the docker binary in ``/usr/local/bin``

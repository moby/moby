:title: Requirements and Installation on Red Hat Enterprise Linux / CentOS
:description: Please note this project is currently under heavy development. It should not be used in production.
:keywords: Docker, Docker documentation, requirements, linux, rhel, centos

.. _rhel:

Red Hat Enterprise Linux / CentOS
=================================

.. include:: install_header.inc

.. include:: install_unofficial.inc

Docker is available for **RHEL/CentOS 6**.

Please note that this package is part of a `Extra Packages for Enterprise Linux (EPEL)`_, a community effort to create and maintain additional packages for RHEL distribution.

Please note that due to the current Docker limitations Docker is able to run only on the **64 bit** architecture.

Installation
------------

1. Firstly, let's make sure our RHEL host is up-to-date.

.. code-block:: bash

    sudo yum -y upgrade

2. Next you need to install the EPEL repository. Please follow the `EPEL installation instructions`_.

3. Next let's install the ``docker-io`` package which will install Docker on our host.

.. code-block:: bash

   sudo yum -y install docker-io

4. Now it's installed lets start the Docker daemon.

.. code-block:: bash

    sudo service docker start

If we want Docker to start at boot we should also:

.. code-block:: bash

   sudo chkconfig docker on

5. Now let's verify that Docker is working.

.. code-block:: bash

   sudo docker run -i -t ubuntu /bin/bash

**Done!**, now continue with the :ref:`hello_world` example.

Issues?
-------

If you have any issues - please report them directly in the `Red Hat Bugzilla for docker-io component`_.

.. _Extra Packages for Enterprise Linux (EPEL): https://fedoraproject.org/wiki/EPEL
.. _EPEL installation instructions: https://fedoraproject.org/wiki/EPEL#How_can_I_use_these_extra_packages.3F
.. _Red Hat Bugzilla for docker-io component : https://bugzilla.redhat.com/enter_bug.cgi?product=Fedora%20EPEL&component=docker-io


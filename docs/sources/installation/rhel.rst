:title: Requirements and Installation on Red Hat Enterprise Linux
:description: Please note this project is currently under heavy development. It should not be used in production.
:keywords: Docker, Docker documentation, requirements, linux, rhel, centos

.. _rhel:

Red Hat Enterprise Linux 
========================

.. include:: install_header.inc

.. include:: install_unofficial.inc

Docker is available for **RHEL** on EPEL. These instructions should work for
both RHEL and CentOS. They will likely work for other binary compatible EL6
distributions as well, but they haven't been tested.

Please note that this package is part of `Extra Packages for Enterprise
Linux (EPEL)`_, a community effort to create and maintain additional packages
for the RHEL distribution.

Also note that due to the current Docker limitations, Docker is able to run
only on the **64 bit** architecture.

Installation
------------

1. Firstly, you need to install the EPEL repository. Please follow the `EPEL installation instructions`_.

2. Next, let's install the ``docker-io`` package which will install Docker on our host.

.. code-block:: bash

   sudo yum -y install docker-io

To keep docker up to date:

.. code-block:: bash

   sudo yum -y update docker-io

3. Now that it's installed, lets start the Docker daemon.

.. code-block:: bash

    sudo service docker start

If we want Docker to start at boot we should also:

.. code-block:: bash

   sudo chkconfig docker on

4. Now let's verify that Docker is working.

.. code-block:: bash

   sudo docker run -i -t mattdm/fedora /bin/bash

**Done!**, now continue with the :ref:`hello_world` example.

Issues?
-------

If you have any issues - please report them directly in the `Red Hat Bugzilla for docker-io component`_.

.. _Extra Packages for Enterprise Linux (EPEL): https://fedoraproject.org/wiki/EPEL
.. _EPEL installation instructions: https://fedoraproject.org/wiki/EPEL#How_can_I_use_these_extra_packages.3F
.. _Red Hat Bugzilla for docker-io component : https://bugzilla.redhat.com/enter_bug.cgi?product=Fedora%20EPEL&component=docker-io


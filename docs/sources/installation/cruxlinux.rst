:title: Installation on CRUX Linux
:description: Docker installation on CRUX Linux.
:keywords: crux linux, virtualization, Docker, documentation, installation

.. _crux_linux:


CRUX Linux
==========

.. include:: install_header.inc

.. include:: install_unofficial.inc

Installing on CRUX Linux can be handled via the ports from `James Mills <http://prologic.shortcircuit.net.au/>`_:

* `docker <https://bitbucket.org/prologic/ports/src/tip/docker/>`_

* `docker-bin <https://bitbucket.org/prologic/ports/src/tip/docker-bin/>`_

* `docker-git <https://bitbucket.org/prologic/ports/src/tip/docker-git/>`_

The ``docker`` port will install the latest tagged version of Docker. 
The ``docker-bin`` port will install the latest tagged versin of Docker from upstream built binaries.
The ``docker-git`` package will build from the current master branch.


Installation
------------

For the time being (*until the CRUX Docker port(s) get into the official contrib repository*) you will need to install
`James Mills' <https://bitbucket.org/prologic/ports>`_ ports repository. You can do so via:

Download the ``httpup`` file to ``/etc/ports/``:
::
    
    curl -q -o - http://crux.nu/portdb/?a=getup&q=prologic > /etc/ports/prologic.httpup
    

Add ``prtdir /usr/ports/prologic`` to ``/etc/prt-get.conf``:
::
    
    vim /etc/prt-get.conf

    # or:
    echo "prtdir /usr/ports/prologic" >> /etc/prt-get.conf
    

Update ports and prt-get cache:
::
    
    ports -u
    prt-get cache
        

To install (*and its dependencies*):
::

    prt-get depinst docker
    

Use ``docker-bin`` for the upstream binary or ``docker-git`` to build and install from the master branch from git.


Kernel Requirements
-------------------

To have a working **CRUX+Docker** Host you must ensure your Kernel
has the necessary modules enabled for LXC containers to function
correctly and Docker Daemon to work properly.

Please read the ``README.rst``:
::
    
    prt-get readme docker
    
There is a ``test_kernel_config.sh`` script in the above ports which you can use to test your Kernel configuration:

::
    
    cd /usr/ports/prologic/docker
    ./test_kernel_config.sh /usr/src/linux/.config
    

Starting Docker
---------------

There is a rc script created for Docker. To start the Docker service:

::
    
    sudo su -
    /etc/rc.d/docker start
    
To start on system boot:

- Edit ``/etc/rc.conf``
- Put ``docker`` into the ``SERVICES=(...)`` array after ``net``.

:title: Host Integration
:description: How to generate scripts for upstart, systemd, etc.
:keywords: systemd, upstart, supervisor, docker, documentation, host integration



Host Integration
================

You can use your Docker containers with process managers like ``upstart``,
``systemd`` and ``supervisor``.

Introduction
------------

When you have finished setting up your image and are happy with your
running container, you may want to use a process manager to manage
it. To help with this, we provide a simple image: ``creack/manger:min``

This image takes the container ID as parameter. We also can specify
the kind of process manager and metadata like *Author* and
*Description*. The output will will be text suitable for a
configuration file, echoed to stdout. It is up to you to create the
.conf file (for `upstart
<http://upstart.ubuntu.com/cookbook/#job-configuration-file>`_) or
.service file (for `systemd
<http://0pointer.de/public/systemd-man/systemd.service.html>`_) and
put it in the right place for your system.

Usage
-----

.. code-block:: bash

   docker run creack/manager:min [OPTIONS] <container id>

.. program:: docker run creack/manager:min

.. cmdoption:: -a="<none>" 

   Author of the image

.. cmdoption:: -d="<none>"

   Description of the image

.. cmdoption:: -t="upstart" 

   Type of manager requested: ``upstart`` or ``systemd``

Example Output
..............

.. code-block:: bash

   docker run creack/manager:min -t="systemd" b28605f2f9a4
   [Unit]
   	Description=<none>
   	Author=<none>
   	After=docker.service

   [Service]
   	Restart=always
   	ExecStart=/usr/bin/docker start -a b28605f2f9a4
   	ExecStop=/usr/bin/docker stop -t 2 b28605f2f9a4

   [Install]
   	WantedBy=local.target



Development
-----------

The image ``creack/manager:min`` is a ``busybox`` base with the
compiled binary of ``manager.go`` as the :ref:`Entrypoint
<entrypoint_def>`.  It is meant to be light and fast to download.

If you would like to change or add things, you can download the full
``creack/manager`` repository that contains ``creack/manager:min`` and
``creack/manager:dev``.

The Dockerfiles and the sources are available in
`/contrib/host_integration
<https://github.com/dotcloud/docker/tree/master/contrib/host_integration>`_.


Upstart
-------

Upstart is the default process manager. The generated script will
start the container after the ``docker`` daemon. If the container
dies, it will respawn.  Start/Restart/Stop/Reload are
supported. Reload will send a SIGHUP to the container.

Example (``upstart`` on Debian)
...............................

.. code-block:: bash

   CID=$(docker run -d creack/firefo-vnc)
   docker run creack/manager:min -a 'Guillaume J. Charmes <guillaume@dotcloud.com>' -d 'Awesome Firefox in VLC' $CID > /etc/init/firefoxvnc.conf

You can now ``start firefoxvnc`` or ``stop firefoxvnc`` and if the container
dies for some reason, upstart will restart it.

Systemd
-------

In order to generate a systemd script, we need to use the ``-t``
option. The generated script will start the container after docker
daemon. If the container dies, it will respawn.
``Start/Restart/Reload/Stop`` are supported.

Example (``systemd`` on Fedora)
...............................

.. code-block:: bash

   CID=$(docker run -d creack/firefo-vnc)
   docker run creack/manager:min -t systemd -a 'Guillaume J. Charmes <guillaume@dotcloud.com>' -d 'Awesome Firefox in VLC' $CID > /usr/lib/systemd/system/firefoxvnc.service

You can now run ``systemctl start firefoxvnc`` or ``systemctl stop
firefoxvnc`` and if the container dies for some reason, ``systemd``
will restart it.

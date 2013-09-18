:title: Port redirection
:description: usage about port redirection
:keywords: Usage, basic port, docker, documentation, examples


.. _port_redirection:

Port redirection
================

Docker can redirect public TCP and UDP ports to your container, so it can be
reached over the network.  Port redirection is done on ``docker run``
using the -p flag.

A port redirect is specified as *PUBLIC:PRIVATE*, where TCP port
*PUBLIC* will be redirected to TCP port *PRIVATE*. As a special case,
the public port can be omitted, in which case a random public port
will be allocated.

.. code-block:: bash

    # A random PUBLIC port is redirected to PRIVATE port 80 on the container
    sudo docker run -p 80 <image> <cmd>

    # PUBLIC port 80 is redirected to PRIVATE port 80
    sudo docker run -p 80:80 <image> <cmd>

To redirect a UDP port the redirection must be expressed as *PUBLIC:PRIVATE/udp*:

.. code-block:: bash

    # PUBLIC port 5300 is redirected to the PRIVATE port 53 using UDP
    sudo docker run -p 5300:53/udp <image> <cmd>

Default port redirects can be built into a container with the
``EXPOSE`` build command.

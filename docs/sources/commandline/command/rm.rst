:title: Rm Command
:description: Remove a container
:keywords: remove, container, docker, documentation, rm

============================
``rm`` -- Remove a container
============================

::

    Usage: docker rm [OPTIONS] CONTAINER

    Remove one or more containers
        -link="": Remove the link instead of the actual container
 

Examples:
--------

.. code-block:: bash

    $ docker rm /redis
    /redis


This will remove the container referenced under the link ``/redis``.


.. code-block:: bash

    $ docker rm -link /webapp/redis
    /webapp/redis


This will remove the underlying link between ``/webapp`` and the ``/redis`` containers removing all
network communication.

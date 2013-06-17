:title: Port redirection
:description: usage about port redirection
:keywords: Usage, basic port, docker, documentation, examples


Port redirection
================

Port redirection is done on ``docker run`` using the -p flag.

Here are the 3 ways to redirect a port:

.. code-block:: bash

    # the port 80 in the container is mapped to a random port of the host
    docker run -p 80 <image> <cmd>

    # the port 80 in the container is mapped to the port 80 of the host
    docker run -p :80 <image> <cmd>

    # the port 80 in the container is mapped to the port 5555 of the host
    docker run -p 5555:80 <image <cmd>


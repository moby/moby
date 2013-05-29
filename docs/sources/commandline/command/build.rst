:title: Build Command
:description: Build a new image from the Dockerfile passed via stdin
:keywords: build, docker, container, documentation

================================================
``build`` -- Build a container from a Dockerfile
================================================

::

    Usage: docker build [CONTEXT|-]
    Build a new image from a Dockerfile

Examples
--------

.. code-block:: bash

    docker build

This will take the local Dockerfile without context

.. code-block:: bash

    docker build -

This will read a Dockerfile form Stdin without context

.. code-block:: bash

    docker build .

This will take the local Dockerfile and set the current directory as context

.. code-block:: bash

    docker build - .

This will read a Dockerfile from Stdin and set the current directory as context

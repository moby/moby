:title: Build Command
:description: Build a new image from the Dockerfile passed via stdin
:keywords: build, docker, container, documentation

================================================
``build`` -- Build a container from a Dockerfile
================================================

::

    Usage: docker build [OPTIONS] PATH | -
    Build a new container image from the source code at PATH
      -t="": Tag to be applied to the resulting image in case of success.

Examples
--------

.. code-block:: bash

    docker build .

This will take the local Dockerfile

.. code-block:: bash

    docker build -

This will read a Dockerfile form Stdin without context

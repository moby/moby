:title: Build Command
:description: Build a new image from the Dockerfile passed via stdin
:keywords: build, docker, container, documentation

================================================
``build`` -- Build a container from a Dockerfile
================================================

::

    Usage: docker build [OPTIONS] PATH | URL | -
    Build a new container image from the source code at PATH
      -t="": Repository name (and optionally a tag) to be applied to the resulting image in case of success.
      -q=false: Suppress verbose build output.
      -no-cache: Do not use the cache when building the image.
      -rm: Remove intermediate containers after a successful build
    When a single Dockerfile is given as URL, then no context is set. When a git repository is set as URL, the repository is used as context


Examples
--------

.. code-block:: bash

    sudo docker build .

This will read the ``Dockerfile`` from the current directory. It will
also send any other files and directories found in the current
directory to the ``docker`` daemon.

The contents of this directory would be used by ``ADD`` commands found
within the ``Dockerfile``.  This will send a lot of data to the
``docker`` daemon if the current directory contains a lot of data.  If
the absolute path is provided instead of ``.`` then only the files and
directories required by the ADD commands from the ``Dockerfile`` will be
added to the context and transferred to the ``docker`` daemon.

.. code-block:: bash

   sudo docker build -t vieux/apache:2.0 .

This will build like the previous example, but it will then tag the
resulting image. The repository name will be ``vieux/apache`` and the
tag will be ``2.0``


.. code-block:: bash

    sudo docker build - < Dockerfile

This will read a ``Dockerfile`` from *stdin* without context. Due to
the lack of a context, no contents of any local directory will be sent
to the ``docker`` daemon.  ``ADD`` doesn't work when running in this
mode because the absence of the context provides no source files to
copy to the container.


.. code-block:: bash

    sudo docker build github.com/creack/docker-firefox

This will clone the Github repository and use it as context. The
``Dockerfile`` at the root of the repository is used as
``Dockerfile``.  Note that you can specify an arbitrary git repository
by using the ``git://`` schema.

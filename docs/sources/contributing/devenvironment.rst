:title: Setting Up a Dev Environment
:description: Guides on how to contribute to docker
:keywords: Docker, documentation, developers, contributing, dev environment

Setting Up a Dev Environment
============================

To make it easier to contribute to Docker, we provide a standard development environment. It is important that
the same environment be used for all tests, builds and releases. The standard development environment defines
all build dependencies: system libraries and binaries, go environment, go dependencies, etc.


Step 1: install docker
----------------------

Docker's build environment itself is a docker container, so the first step is to install docker on your system.

You can follow the `install instructions most relevant to your system <https://docs.docker.io/en/latest/installation/>`.
Make sure you have a working, up-to-date docker installation, then continue to the next step.


Step 2: check out the source
----------------------------

::

    git clone http://git@github.com/dotcloud/docker
    cd docker


Step 3: build
-------------

When you are ready to build docker, run this command:

::

    docker build -t docker .

This will build the revision currently checked out in the repository. Feel free to check out the version
of your choice.

If the build is successful, congratulations! You have produced a clean build of docker, neatly encapsulated
in a standard build environment.

You can run an interactive session in the newly built container:

::

    docker run -i -t docker bash


To extract the binaries from the container:

::

    docker run docker sh -c 'cat $(which docker)' > docker-build && chmod +x docker-build


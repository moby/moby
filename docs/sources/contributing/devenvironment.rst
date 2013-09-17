:title: Setting Up a Dev Environment
:description: Guides on how to contribute to docker
:keywords: Docker, documentation, developers, contributing, dev environment

Setting Up a Dev Environment
============================

To make it easier to contribute to Docker, we provide a standard
development environment. It is important that the same environment be
used for all tests, builds and releases. The standard development
environment defines all build dependencies: system libraries and
binaries, go environment, go dependencies, etc.


Step 1: Install Docker
----------------------

Docker's build environment itself is a Docker container, so the first
step is to install Docker on your system.

You can follow the `install instructions most relevant to your system
<https://docs.docker.io/en/latest/installation/>`_.  Make sure you have
a working, up-to-date docker installation, then continue to the next
step.


Step 2: Check out the Source
----------------------------

::

    git clone http://git@github.com/dotcloud/docker
    cd docker

To checkout a different revision just use ``git checkout`` with the name of branch or revision number.


Step 3: Build Docker
---------------------

When you are ready to build docker, run this command:

::

    sudo docker build -t docker .

This will build a container using the Dockerfile in the current directory. Essentially, it will install all the build and runtime dependencies necessary to build and test docker. This command will take some time to complete when you execute it.


If the build is successful, congratulations! You have produced a clean build of docker, neatly encapsulated in a standard build environment. 


Step 4: Testing the Docker Build
---------------------------------

If you have successfully complete the previous steps then you can test the Docker build by executing the following command

::

	sudo docker run -lxc-conf=lxc.aa_profile=unconfined -privileged -v `pwd`:/go/src/github.com/dotcloud/docker docker hack/make.sh test


Step 5: Use Docker
-------------------

You can run an interactive session in the newly built container: 

::

	sudo docker run -privileged -i -t docker bash

To exit the interactive session simply type ``exit``.


To extract the binaries from the container:

::

    sudo docker run docker sh -c 'cat $(which docker)' > docker-build && chmod +x docker-build


Need More Help?
===============

If you need more help then hop on to the #docker-dev IRC channel.

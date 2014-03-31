:title: Running a Riak service
:description: Build a Docker image with Riak pre-installed
:keywords: docker, example, package installation, networking, riak

Riak Service
==============================

.. include:: example_header.inc

The goal of this example is to show you how to build a Docker image with Riak
pre-installed.

Creating a ``Dockerfile``
+++++++++++++++++++++++++

Create an empty file called ``Dockerfile``:

.. code-block:: bash

    touch Dockerfile

Next, define the parent image you want to use to build your image on top of.
We’ll use `Ubuntu <https://index.docker.io/_/ubuntu/>`_ (tag: ``latest``),
which is available on the `docker index <http://index.docker.io>`_:

.. code-block:: bash

    # Riak
    #
    # VERSION       0.1.0

    # Use the Ubuntu base image provided by dotCloud
    FROM ubuntu:latest
    MAINTAINER Hector Castro hector@basho.com

Next, we update the APT cache and apply any updates:

.. code-block:: bash

    # Update the APT cache
    RUN sed -i.bak 's/main$/main universe/' /etc/apt/sources.list
    RUN apt-get update
    RUN apt-get upgrade -y

After that, we install and setup a few dependencies:

- ``curl`` is used to download Basho's APT repository key
- ``lsb-release`` helps us derive the Ubuntu release codename
- ``openssh-server`` allows us to login to containers remotely and join Riak
  nodes to form a cluster
- ``supervisor`` is used manage the OpenSSH and Riak processes

.. code-block:: bash

    # Install and setup project dependencies
    RUN apt-get install -y curl lsb-release supervisor openssh-server

    RUN mkdir -p /var/run/sshd
    RUN mkdir -p /var/log/supervisor

    RUN locale-gen en_US en_US.UTF-8

    ADD supervisord.conf /etc/supervisor/conf.d/supervisord.conf

    RUN echo 'root:basho' | chpasswd

Next, we add Basho's APT repository:

.. code-block:: bash

    RUN curl -s http://apt.basho.com/gpg/basho.apt.key | apt-key add --
    RUN echo "deb http://apt.basho.com $(lsb_release -cs) main" > /etc/apt/sources.list.d/basho.list
    RUN apt-get update

After that, we install Riak and alter a few defaults:

.. code-block:: bash

    # Install Riak and prepare it to run
    RUN apt-get install -y riak
    RUN sed -i.bak 's/127.0.0.1/0.0.0.0/' /etc/riak/app.config
    RUN echo "ulimit -n 4096" >> /etc/default/riak

Almost there. Next, we add a hack to get us by the lack of ``initctl``:

.. code-block:: bash

    # Hack for initctl
    # See: https://github.com/dotcloud/docker/issues/1024
    RUN dpkg-divert --local --rename --add /sbin/initctl
    RUN ln -sf /bin/true /sbin/initctl

Then, we expose the Riak Protocol Buffers and HTTP interfaces, along with SSH:

.. code-block:: bash

    # Expose Riak Protocol Buffers and HTTP interfaces, along with SSH
    EXPOSE 8087 8098 22

Finally, run ``supervisord`` so that Riak and OpenSSH are started:

.. code-block:: bash

    CMD ["/usr/bin/supervisord"]

Create a ``supervisord`` configuration file
+++++++++++++++++++++++++++++++++++++++++++

Create an empty file called ``supervisord.conf``. Make sure it's at the same
directory level as your ``Dockerfile``:

.. code-block:: bash

    touch supervisord.conf

Populate it with the following program definitions:

.. code-block:: bash

    [supervisord]
    nodaemon=true

    [program:sshd]
    command=/usr/sbin/sshd -D
    stdout_logfile=/var/log/supervisor/%(program_name)s.log
    stderr_logfile=/var/log/supervisor/%(program_name)s.log
    autorestart=true

    [program:riak]
    command=bash -c ". /etc/default/riak && /usr/sbin/riak console"
    pidfile=/var/log/riak/riak.pid
    stdout_logfile=/var/log/supervisor/%(program_name)s.log
    stderr_logfile=/var/log/supervisor/%(program_name)s.log

Build the Docker image for Riak
+++++++++++++++++++++++++++++++

Now you should be able to build a Docker image for Riak:

.. code-block:: bash

    docker build -t "<yourname>/riak" .

Next steps
++++++++++

Riak is a distributed database. Many production deployments consist of `at
least five nodes <http://basho.com/why-your-riak-cluster-should-have-at-least-
five-nodes/>`_. See the `docker-riak <https://github.com/hectcastro /docker-
riak>`_ project details on how to deploy a Riak cluster using Docker and
Pipework.

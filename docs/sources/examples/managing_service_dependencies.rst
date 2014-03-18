:title: Managing Service Dependencies
:description: Manage containers that rely on other containers
:keywords: docker, example, service dependencies

.. _managing_service_dependencies:

Creating Dependent Services
===========================

.. include:: example_header.inc

In many cases a container relies on services provided by other containers:
a database, central authentication system, or other service.
This requires us to make sure the depended-on containers are running, probably
before the container that calls them is running.


Linked Containers
+++++++++++++++++

Docker provides a mechanism to link containers as a means of service discovery and
inter-container communication.

.. code-block:: bash

  docker run -d --name foo busybox
  docker run -d --link foo:foo busybox


Using container links for service depencies
+++++++++++++++++++++++++++++++++++++++++++

You can now also use these links to ensure dependent containers are started before
your main container is, without leaving Docker.

.. code-block:: bash

  docker run -d --name foo busybox sleep 300
  docker run -d --name bar --link foo:foo busybox sleep 300

  docker kill foo
  docker kill bar

This creates two containers and links them together, then stops them, now lets fire
them up.
Normally you would start the "foo" container first, then the "bar" container, but
here is what we will do instead:

.. code-block:: bash

  docker start --cascade bar

The "--cascade" option will tell Docker to start the "foo" before it starts the
"bar" container.

Using this option you can always ensure that any and all containers your container
depends on will be started first.


Pretty nifty.

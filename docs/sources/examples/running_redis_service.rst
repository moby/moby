:title: Running a Redis service
:description: Installing and running an redis service
:keywords: docker, example, package installation, networking, redis

.. _running_redis_service:

Create a redis service
======================

Very simple, no frills, redis service.

This example assumes you have Docker installed and the base image already
imported.

Open a docker container
-----------------------

::

    $ docker run -i -t base /bin/bash

Building your image
-------------------

Within your docker container.  Once installed, <ctl-c> out of docker.

::

    $ apt-get update
    $ apt-get install redis-server
    SIGINT received

Snapshot the installation
-------------------------

::

    $ docker ps   # grab the container id
    $ docker commit <container_id> <your username>/redis

Run the service
---------------

Running the service with `-d` runs the container in detached mode, leaving the
container running in the background.
::

    $ docker run -d -p 6379 -i -t <your username>/redis /usr/bin/redis-server

Test
----

::

    $ docker ps  # grab the new container id
    $ docker inspect <container_id>    # grab the ipaddress
    $ docker port <container_id> 6379  # grab the external port
    $ redis-cli -h <ipaddress> -p <external port>
    redis 10.0.3.32:49175> set docker awesome
    OK
    redis 10.0.3.32:49175> get docker
    "awesome"

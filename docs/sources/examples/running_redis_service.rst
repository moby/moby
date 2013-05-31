:title: Running a Redis service
:description: Installing and running an redis service
:keywords: docker, example, package installation, networking, redis

.. _running_redis_service:

Create a redis service
======================

.. include:: example_header.inc

Very simple, no frills, redis service.

Open a docker container
-----------------------

.. code-block:: bash

    docker run -i -t base /bin/bash

Building your image
-------------------

Update your docker container, install the redis server.  Once installed, exit out of docker.

.. code-block:: bash

    apt-get update
    apt-get install redis-server
    exit

Snapshot the installation
-------------------------

.. code-block:: bash

    docker ps -a  # grab the container id (this will be the first one in the list)
    docker commit <container_id> <your username>/redis

Run the service
---------------

Running the service with `-d` runs the container in detached mode, leaving the
container running in the background. Use your snapshot.

.. code-block:: bash

    docker run -d -p 6379 <your username>/redis /usr/bin/redis-server

Test 1
++++++

Connect to the container with the redis-cli.

.. code-block:: bash

    docker ps  # grab the new container id
    docker inspect <container_id>    # grab the ipaddress of the container
    redis-cli -h <ipaddress> -p 6379
    redis 10.0.3.32:6379> set docker awesome
    OK
    redis 10.0.3.32:6379> get docker
    "awesome"
    redis 10.0.3.32:6379> exit

Test 2
++++++

Connect to the host os with the redis-cli.

.. code-block:: bash

    docker ps  # grab the new container id
    docker port <container_id> 6379  # grab the external port
    ifconfig   # grab the host ip address
    redis-cli -h <host ipaddress> -p <external port>
    redis 192.168.0.1:49153> set docker awesome
    OK
    redis 192.168.0.1:49153> get docker
    "awesome"
    redis 192.168.0.1:49153> exit

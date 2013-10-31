:title: Linking to an Redis container
:description: Running redis linked into your web app
:keywords: docker, example, networking, redis, link

.. _linking_redis:

Linking Redis
=============

.. include:: example_header.inc

Building a redis container to link as a child of our web application.

Building the redis container
----------------------------

Lets build a redis image with the following Dockerfile.

.. code-block:: bash

    git clone https://github.com/antirez/redis.git
    cd redis
    git checkout 2.6

    # Save this Dockerfile to the root of the redis repository.  

    # Build redis from source
    # Make sure you have the redis source code checked out in
    # the same directory as this Dockerfile
    FROM ubuntu

    RUN echo "deb http://archive.ubuntu.com/ubuntu precise main universe" > /etc/apt/sources.list
    RUN apt-get update
    RUN apt-get upgrade -y

    RUN apt-get install -y gcc make g++ build-essential libc6-dev tcl

    ADD . /redis

    RUN (cd /redis && make)

    RUN mkdir -p /redis-data
    VOLUME ["/redis-data"]
    EXPOSE 6379

    ENTRYPOINT ["/redis/src/redis-server"]
    CMD ["--dir", "/redis-data"]

    # docker build our new redis image from source
    docker build -t redis-2.6 .


We need to ``EXPOSE`` the default port of 6379 so that our link knows what ports 
to connect to our redis container on.  If you do not expose any ports for the
image then docker will not be able to establish the link between containers.


Run the redis container
-----------------------

.. code-block:: bash
    
    docker run -d -e PASSWORD=docker -name redis redis-2.6 --requirepass docker
 
This will run our redis container with the password docker 
to secure our service.  By specifying the ``-name`` flag on run 
we will assign the name ``redis`` to this container.  If we do not specify a name  for 
our container via the ``-name`` flag docker will automatically generate a name for us.
We can issue all the commands that you would expect; start, stop, attach, using the name for our container.
The name also allows us to link other containers into this one.

Linking redis as a child
------------------------

Next we can start a new web application that has a dependency on redis and apply a link 
to connect both containers.  If you noticed when running our redis server we did not use
the ``-p`` flag to publish the redis port to the host system.  Redis exposed port 6379 via the Dockerfile 
and this is all we need to establish a link.

Now lets start our web application with a link into redis.

.. code-block:: bash
   
    docker run -t -i -link redis:db -name webapp ubuntu bash

    root@4c01db0b339c:/# env

    HOSTNAME=4c01db0b339c
    DB_NAME=/webapp/db
    TERM=xterm
    DB_PORT=tcp://172.17.0.8:6379
    DB_PORT_6379_TCP=tcp://172.17.0.8:6379
    DB_PORT_6379_TCP_PROTO=tcp
    DB_PORT_6379_TCP_ADDR=172.17.0.8
    DB_PORT_6379_TCP_PORT=6379
    PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
    PWD=/
    DB_ENV_PASSWORD=dockerpass
    SHLVL=1
    HOME=/
    container=lxc
    _=/usr/bin/env
    root@4c01db0b339c:/#


When we inspect the environment of the linked container we can see a few extra environment 
variables have been added.  When you specified ``-link redis:db`` you are telling docker
to link the container named ``redis`` into this new container with the alias ``db``.  
Environment variables are prefixed with the alias so that the parent container can access
network and environment information from the containers that are linked into it.

.. code-block:: bash

    # The name of the child container
    DB_NAME=/webapp/db

    # The default protocol, ip, and port of the service running in the container
    DB_PORT=tcp://172.17.0.8:6379

    # A specific protocol, ip, and port of various services
    DB_PORT_6379_TCP=tcp://172.17.0.8:6379
    DB_PORT_6379_TCP_PROTO=tcp
    DB_PORT_6379_TCP_ADDR=172.17.0.8
    DB_PORT_6379_TCP_PORT=6379

    # Get environment variables of the container 
    DB_ENV_PASSWORD=dockerpass


Accessing the network information along with the environment of the child container allows
us to easily connect to the redis service on the specific ip and port and use the password
specified in the environment.

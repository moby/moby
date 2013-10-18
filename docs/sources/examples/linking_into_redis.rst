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

We will use a pre-build version of redis from the index under 
the name ``crosbymichael/redis``.  If you are interested in the 
Dockerfile that was used to build this container here it is.

.. code-block:: bash
    
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
    RUN (cd /redis && make test)

    RUN mkdir -p /redis-data
    VOLUME ["/redis-data"]
    EXPOSE 6379

    ENTRYPOINT ["/redis/src/redis-server"]
    CMD ["--dir", "/redis-data"]


We need to ``EXPOSE`` the default port of 6379 so that our link knows what ports 
to connect to our redis container on.  If you do not expose any ports for the
image then docker will not be able to establish the link between containers.


Run the redis container
-----------------------

.. code-block:: bash
    
    docker run -d -e PASSWORD=docker crosbymichael/redis --requirepass docker
 
This will run our redis container using the default port of 6379 and using
as password to secure our service. Next we will link the redis container to 
a new name using ``docker link``.


Linking an existing container
-----------------------------

Docker will automatically create an initial link with the container's id but
because the is long and not very user friendly we can link the container with
a new name.

.. code-block:: bash

    docker link /39588b6a45100ef5b328b2c302ea085624f29e6cbab70f88be04793af02cec89 /redis

Now we can reference our running redis service using the friendly name ``/redis``.  
We can issue all the commands that you would expect; start, stop, attach, using the new name.

Linking redis as a child
------------------------

Next we can start a new web application that has a dependency on redis and apply a link 
to connect both containers.  If you noticed when running our redis service we did not use
the ``-p`` option to publish the redis port to the host system.  Redis exposed port 6379
but we did not publish the port.  This allows docker to prevent all network traffic to
the redis container except when explicitly specified within a link.  This is a big win
for security.  


Now lets start our web application with a link into redis.

.. code-block:: bash
   
    docker run -t -i -link /redis:db ubuntu bash

    root@4c01db0b339c:/# env

    HOSTNAME=4c01db0b339c
    DB_NAME=/4c01db0b339cf19958731255a796ee072040a652f51652a4ade190ab8c27006f/db
    TERM=xterm
    DB_PORT=tcp://172.17.0.8:6379
    DB_PORT_6379_TCP=tcp://172.17.0.8:6379
    PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
    PWD=/
    DB_ENV_PASSWORD=dockerpass
    SHLVL=1
    HOME=/
    container=lxc
    _=/usr/bin/env
    root@4c01db0b339c:/#


When we inspect the environment of the linked container we can see a few extra environment 
variables have been added.  When you specified ``-link /redis:db`` you are telling docker
to link the container named ``/redis`` into this new container with the alias ``db``.  
Environment variables are prefixed with the alias so that the parent container can access
network and environment information from the child.

.. code-block:: bash

    # The name of the child container
    DB_NAME=/4c01db0b339cf19958731255a796ee072040a652f51652a4ade190ab8c27006f/db
    # The default protocol, ip, and port of the service running in the container
    DB_PORT=tcp://172.17.0.8:6379
    # A specific protocol, ip, and port of various services
    DB_PORT_6379_TCP=tcp://172.17.0.8:6379
    # Get environment variables of the container 
    DB_ENV_PASSWORD=dockerpass


Accessing the network information along with the environment of the child container allows
us to easily connect to the redis service on the specific ip and port and use the password
specified in the environment.  

:title: PostgreSQL service How-To
:description: Running and installing a PostgreSQL service
:keywords: docker, example, package installation, postgresql

.. _postgresql_service:

PostgreSQL Service
==================

.. include:: example_header.inc

Installing PostgreSQL on Docker
-------------------------------

Assuming there is no Docker image that suits your needs in `the index`_, you 
can create one yourself.

.. _the index: http://index.docker.io

Start by creating a new Dockerfile:

.. note::

    This PostgreSQL setup is for development only purposes. Refer
    to the PostgreSQL documentation to fine-tune these settings so that it
    is suitably secure.

.. literalinclude:: postgresql_service.Dockerfile

Build an image from the Dockerfile assign it a name.

.. code-block:: bash

    $ sudo docker build -t eg_postgresql .

And run the PostgreSQL server container (in the foreground):

.. code-block:: bash

    $ sudo docker run --rm -P --name pg_test eg_postgresql

There are  2 ways to connect to the PostgreSQL server. We can use 
:ref:`working_with_links_names`, or we can access it from our host (or the network).

.. note:: The ``--rm`` removes the container and its image when the container 
          exists successfully.

Using container linking
^^^^^^^^^^^^^^^^^^^^^^^

Containers can be linked to another container's ports directly using 
``--link remote_name:local_alias`` in the client's ``docker run``. This will
set a number of environment variables that can then be used to connect:

.. code-block:: bash

    $ sudo docker run --rm -t -i --link pg_test:pg eg_postgresql bash

    postgres@7ef98b1b7243:/$ psql -h $PG_PORT_5432_TCP_ADDR -p $PG_PORT_5432_TCP_PORT -d docker -U docker --password

Connecting from your host system
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Assuming you have the postgresql-client installed, you can use the host-mapped port
to test as well. You need to use ``docker ps`` to find out what local host port the 
container is mapped to first:

.. code-block:: bash

    $ docker ps
    CONTAINER ID        IMAGE                  COMMAND                CREATED             STATUS              PORTS                                      NAMES
    5e24362f27f6        eg_postgresql:latest   /usr/lib/postgresql/   About an hour ago   Up About an hour    0.0.0.0:49153->5432/tcp                    pg_test
    $ psql -h localhost -p 49153 -d docker -U docker --password

Testing the database
^^^^^^^^^^^^^^^^^^^^

Once you have authenticated and have a ``docker =#`` prompt, you can
create a table and populate it.

.. code-block:: bash

    psql (9.3.1)
    Type "help" for help.

    docker=# CREATE TABLE cities (
    docker(#     name            varchar(80),
    docker(#     location        point
    docker(# );
    CREATE TABLE
    docker=# INSERT INTO cities VALUES ('San Francisco', '(-194.0, 53.0)');
    INSERT 0 1
    docker=# select * from cities;
         name      | location  
    ---------------+-----------
     San Francisco | (-194,53)
    (1 row)

Using the container volumes
^^^^^^^^^^^^^^^^^^^^^^^^^^^

You can use the defined volumes to inspect the PostgreSQL log files and to backup your
configuration and data:

.. code-block:: bash

    docker run --rm --volumes-from pg_test -t -i busybox sh

    / # ls
    bin      etc      lib      linuxrc  mnt      proc     run      sys      usr
    dev      home     lib64    media    opt      root     sbin     tmp      var
    / # ls /etc/postgresql/9.3/main/
    environment      pg_hba.conf      postgresql.conf
    pg_ctl.conf      pg_ident.conf    start.conf
    /tmp # ls /var/log
    ldconfig    postgresql


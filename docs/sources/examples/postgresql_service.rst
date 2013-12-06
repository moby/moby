:title: PostgreSQL service How-To
:description: Running and installing a PostgreSQL service
:keywords: docker, example, package installation, postgresql

.. _postgresql_service:

PostgreSQL Service
==================

.. include:: example_header.inc

.. note::

    A shorter version of `this blog post`_.

.. _this blog post: http://zaiste.net/2013/08/docker_postgresql_how_to/

Installing PostgreSQL on Docker
-------------------------------

Run an interactive shell in a Docker container.

.. code-block:: bash

    sudo docker run -i -t ubuntu /bin/bash

Update its dependencies.

.. code-block:: bash

    apt-get update

Install ``python-software-properties``, ``software-properties-common``, ``wget`` and ``vim``.

.. code-block:: bash

    apt-get -y install python-software-properties software-properties-common wget vim

Add PostgreSQL's repository. It contains the most recent stable release
of PostgreSQL, ``9.3``.

.. code-block:: bash

    wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -
    echo "deb http://apt.postgresql.org/pub/repos/apt/ precise-pgdg main" > /etc/apt/sources.list.d/pgdg.list
    apt-get update

Finally, install PostgreSQL 9.3

.. code-block:: bash

    apt-get -y install postgresql-9.3 postgresql-client-9.3 postgresql-contrib-9.3

Now, create a PostgreSQL superuser role that can create databases and
other roles.  Following Vagrant's convention the role will be named
``docker`` with ``docker`` password assigned to it.

.. code-block:: bash

    su postgres -c "createuser -P -d -r -s docker"

Create a test database also named ``docker`` owned by previously created ``docker``
role.

.. code-block:: bash

    su postgres -c "createdb -O docker docker"

Adjust PostgreSQL configuration so that remote connections to the
database are possible. Make sure that inside
``/etc/postgresql/9.3/main/pg_hba.conf`` you have following line:

.. code-block:: bash

    host    all             all             0.0.0.0/0               md5

Additionaly, inside ``/etc/postgresql/9.3/main/postgresql.conf``
uncomment ``listen_addresses`` like so:

.. code-block:: bash

    listen_addresses='*'

.. note::

    This PostgreSQL setup is for development only purposes. Refer
    to PostgreSQL documentation how to fine-tune these settings so that it
    is secure enough.

Exit.

.. code-block:: bash

    exit

Create an image from our container and assign it a name. The ``<container_id>``
is in the Bash prompt; you can also locate it using ``docker ps -a``.

.. code-block:: bash

    sudo docker commit <container_id> <your username>/postgresql

Finally, run the PostgreSQL server via ``docker``.

.. code-block:: bash

    CONTAINER=$(sudo docker run -d -p 5432 \
      -t <your username>/postgresql \
      /bin/su postgres -c '/usr/lib/postgresql/9.3/bin/postgres \
        -D /var/lib/postgresql/9.3/main \
        -c config_file=/etc/postgresql/9.3/main/postgresql.conf')

Connect the PostgreSQL server using ``psql`` (You will need the
postgresql client installed on the machine.  For ubuntu, use something
like ``sudo apt-get install postgresql-client``).

.. code-block:: bash

    CONTAINER_IP=$(sudo docker inspect -format='{{.NetworkSettings.IPAddress}}' $CONTAINER)
    psql -h $CONTAINER_IP -p 5432 -d docker -U docker -W

As before, create roles or databases if needed.

.. code-block:: bash

    psql (9.3.1)
    Type "help" for help.

    docker=# CREATE DATABASE foo OWNER=docker;
    CREATE DATABASE

Additionally, publish your newly created image on the Docker Index.

.. code-block:: bash

    sudo docker login
    Username: <your username>
    [...]

.. code-block:: bash

    sudo docker push <your username>/postgresql

PostgreSQL service auto-launch
------------------------------

Running our image seems complicated. We have to specify the whole command with
``docker run``. Let's simplify it so the service starts automatically when the
container starts.

.. code-block:: bash

    sudo docker commit -run='{"Cmd": \
      ["/bin/su", "postgres", "-c", "/usr/lib/postgresql/9.3/bin/postgres -D \
      /var/lib/postgresql/9.3/main -c \
      config_file=/etc/postgresql/9.3/main/postgresql.conf"], "PortSpecs": ["5432"]}' \
      <container_id> <your username>/postgresql

From now on, just type ``docker run <your username>/postgresql`` and
PostgreSQL should automatically start.

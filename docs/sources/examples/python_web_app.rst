:title: Python Web app example
:description: Building your own python web app using docker
:keywords: docker, example, python, web app

.. _python_web_app:

Python Web App
==============

.. include:: example_header.inc

While using Dockerfiles is the prefered way to create maintainable
and repeatable images, its useful to know how you can try things out
and then commit your live changes to an image.

The goal of this example is to show you how you can modify your own
Docker images  by making changes to a running 
container, and then saving the results as a new image. We will do 
that by making a simple 'hello world' Flask web application image.

**Steps:**

.. code-block:: bash

    $ sudo docker pull shykes/pybuilder

Download the ``shykes/pybuilder`` Docker image from the ``http://index.docker.io``
registry. Note that this container was built with a very old version of docker 
(May 2013), but can still be used now.


.. code-block:: bash

    $ sudo docker run -i -t -name pybuilder_run shykes/pybuilder bash

    $$ URL=http://github.com/shykes/helloflask/archive/master.tar.gz
    $$ /usr/local/bin/buildapp $URL
    [lots of output later]
    $$ exit


We then start a new container running interactively  using the
image. 
First, we set a ``URL`` variable that points to a tarball of a simple 
helloflask web app, and then we run a command contained in the image called
``buildapp``, passing it the ``$URL`` variable. The container is
given a name ``pybuilder_run`` which we will use in the next steps.

While this example is simple, you could run any number of interactive commands,
try things out, and then exit when you're done.

.. code-block:: bash

    $ sudo docker commit pybuilder_run /builds/github.com/shykes/helloflask/master
    c8b2e8228f11b8b3e492cbf9a49923ae66496230056d61e07880dc74c5f495f9

Save the changes we just made in the container to a new image called
``/builds/github.com/hykes/helloflask/master``. You now have 3 different
ways to refer to the container, name, short-id ``c8b2e8228f11``, or 
long-id ``c8b2e8228f11b8b3e492cbf9a49923ae66496230056d61e07880dc74c5f495f9``.

.. code-block:: bash

    $ WEB_WORKER=$(sudo docker run -d -p 5000 /builds/github.com/hykes/helloflask/master /usr/local/bin/runapp)

Use the new image to create a new container with
network port 5000, and return the container ID and store in the
``WEB_WORKER`` variable (rather than naming a container/image, you can use the ID's).

- **"docker run -d "** run a command in a new container. We pass "-d"
  so it runs as a daemon.
- **"-p 5000"** the web app is going to listen on this port, so it
  must be mapped from the container to the host system.
- **/usr/local/bin/runapp** is the command which starts the web app.


.. code-block:: bash

    $ sudo docker logs -f $WEB_WORKER
     * Running on http://0.0.0.0:5000/

View the logs for the new container using the ``WEB_WORKER`` variable, and
if everything worked as planned you should see the line ``Running on
http://0.0.0.0:5000/`` in the log output.

To exit the view without stopping the container, hit Ctrl-C, or open another 
terminal and continue with the example while watching the result in the logs.

.. code-block:: bash

    $ WEB_PORT=$(sudo docker port $WEB_WORKER 5000 | awk -F: '{ print $2 }')

Look up the public-facing port which is NAT-ed. Find the private port
used by the container and store it inside of the ``WEB_PORT`` variable.

.. code-block:: bash

    # install curl if necessary, then ...
    $ curl http://127.0.0.1:$WEB_PORT
    Hello world!

Access the web app using the ``curl`` binary. If everything worked as planned you
should see the line ``Hello world!`` inside of your console.

.. code-block:: bash

    $ sudo docker ps --all

List ``--all`` the Docker containers. If this container had already finished
running, it will still be listed here with a status of 'Exit 0'.

.. code-block:: bash

    $ sudo docker stop $WEB_WORKER
    $ sudo docker rm $WEB_WORKER pybuilder_run
    $ sudo docker rmi /builds/github.com/shykes/helloflask/master shykes/pybuilder:latest

And now stop the running web worker, and delete the containers, so that we can 
then delete the images that we used.


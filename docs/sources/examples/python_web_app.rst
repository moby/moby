:title: Python Web app example
:description: Building your own python web app using docker
:keywords: docker, example, python, web app

.. _python_web_app:

Python Web App
==============

.. include:: example_header.inc

The goal of this example is to show you how you can author your own
docker images using a parent image, making changes to it, and then
saving the results as a new image. We will do that by making a simple
hello flask web application image.

**Steps:**

.. code-block:: bash

    sudo docker pull shykes/pybuilder

We are downloading the "shykes/pybuilder" docker image

.. code-block:: bash

    URL=http://github.com/shykes/helloflask/archive/master.tar.gz

We set a URL variable that points to a tarball of a simple helloflask web app

.. code-block:: bash

    BUILD_JOB=$(sudo docker run -d -t shykes/pybuilder:latest /usr/local/bin/buildapp $URL)

Inside of the "shykes/pybuilder" image there is a command called
buildapp, we are running that command and passing the $URL variable
from step 2 to it, and running the whole thing inside of a new
container. BUILD_JOB will be set with the new container_id.

.. code-block:: bash

    sudo docker attach $BUILD_JOB
    [...]

While this container is running, we can attach to the new container to
see what is going on. Ctrl-C to disconnect.

.. code-block:: bash

    sudo docker ps -a
    
List all docker containers. If this container has already finished
running, it will still be listed here.

.. code-block:: bash

    BUILD_IMG=$(sudo docker commit $BUILD_JOB _/builds/github.com/shykes/helloflask/master)

Save the changes we just made in the container to a new image called
``_/builds/github.com/hykes/helloflask/master`` and save the image id in
the BUILD_IMG variable name.

.. code-block:: bash

    WEB_WORKER=$(sudo docker run -d -p 5000 $BUILD_IMG /usr/local/bin/runapp)

- **"docker run -d "** run a command in a new container. We pass "-d"
  so it runs as a daemon.
- **"-p 5000"** the web app is going to listen on this port, so it
  must be mapped from the container to the host system.
- **"$BUILD_IMG"** is the image we want to run the command inside of.
- **/usr/local/bin/runapp** is the command which starts the web app.

Use the new image we just created and create a new container with
network port 5000, and return the container id and store in the
WEB_WORKER variable.

.. code-block:: bash

    sudo docker logs $WEB_WORKER
     * Running on http://0.0.0.0:5000/

View the logs for the new container using the WEB_WORKER variable, and
if everything worked as planned you should see the line "Running on
http://0.0.0.0:5000/" in the log output.

.. code-block:: bash

    WEB_PORT=$(docker port $WEB_WORKER 5000)

Look up the public-facing port which is NAT-ed. Find the private port
used by the container and store it inside of the WEB_PORT variable.

.. code-block:: bash

    # install curl if necessary, then ...
    curl http://127.0.0.1:$WEB_PORT
      Hello world!

Access the web app using curl. If everything worked as planned you
should see the line "Hello world!" inside of your console.

**Video:**

See the example in action

.. raw:: html

    <div style="margin-top:10px;">
      <iframe width="720" height="350" src="http://ascii.io/a/2573/raw" frameborder="0"></iframe>
    </div>

Continue to :ref:`running_ssh_service`.

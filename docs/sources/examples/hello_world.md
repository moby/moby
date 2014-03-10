page_title: Hello world example
page_description: A simple hello world example with Docker
page_keywords: docker, example, hello world

# Check your Docker install

This guide assumes you have a working installation of Docker. To check
your Docker install, run the following command:

    # Check that you have a working install
    $ sudo docker info

If you get `docker: command not found` or something
like `/var/lib/docker/repositories: permission denied`{.docutils
.literal} you may have an incomplete Docker installation or insufficient
privileges to access docker on your machine.

Please refer to [*Installation*](../../installation/#installation-list)
for installation instructions.

# Hello World

Note

-   This example assumes you have Docker running in daemon mode. For
    more information please see [*Check your Docker
    install*](#running-examples).
-   **If you don’t like sudo** then see [*Giving non-root
    access*](../../installation/binaries/#dockergroup)

This is the most basic example available for using Docker.

Download the small base image named `busybox`:

    # Download a busybox image
    $ sudo docker pull busybox

The `busybox` image is a minimal Linux system. You
can do the same with any number of other images, such as
`debian`, `ubuntu`{.docutils .literal} or
`centos`. The images can be found and retrieved
using the [Docker index](http://index.docker.io).

    $ sudo docker run busybox /bin/echo hello world

This command will run a simple `echo` command, that
will echo `hello world` back to the console over
standard out.

**Explanation:**

-   **“sudo”** execute the following commands as user *root*
-   **“docker run”** run a command in a new container
-   **“busybox”** is the image we are running the command in.
-   **“/bin/echo”** is the command we want to run in the container
-   **“hello world”** is the input for the echo command

**Video:**

See the example in action

* * * * *

# Hello World Daemon

Note

-   This example assumes you have Docker running in daemon mode. For
    more information please see [*Check your Docker
    install*](#running-examples).
-   **If you don’t like sudo** then see [*Giving non-root
    access*](../../installation/binaries/#dockergroup)

And now for the most boring daemon ever written!

We will use the Ubuntu image to run a simple hello world daemon that
will just print hello world to standard out every second. It will
continue to do this until we stop it.

**Steps:**

    CONTAINER_ID=$(sudo docker run -d ubuntu /bin/sh -c "while true; do echo hello world; sleep 1; done")

We are going to run a simple hello world daemon in a new container made
from the `ubuntu` image.

-   **“sudo docker run -d “** run a command in a new container. We pass
    “-d” so it runs as a daemon.
-   **“ubuntu”** is the image we want to run the command inside of.
-   **“/bin/sh -c”** is the command we want to run in the container
-   **“while true; do echo hello world; sleep 1; done”** is the mini
    script we want to run, that will just print hello world once a
    second until we stop it.
-   **\$CONTAINER\_ID** the output of the run command will return a
    container id, we can use in future commands to see what is going on
    with this process.

<!-- -->

    sudo docker logs $CONTAINER_ID

Check the logs make sure it is working correctly.

-   **“docker logs**” This will return the logs for a container
-   **\$CONTAINER\_ID** The Id of the container we want the logs for.

<!-- -->

    sudo docker attach -sig-proxy=false $CONTAINER_ID

Attach to the container to see the results in real-time.

-   **“docker attach**” This will allow us to attach to a background
    process to see what is going on.
-   **“-sig-proxy=false”** Do not forward signals to the container;
    allows us to exit the attachment using Control-C without stopping
    the container.
-   **\$CONTAINER\_ID** The Id of the container we want to attach too.

Exit from the container attachment by pressing Control-C.

    sudo docker ps

Check the process list to make sure it is running.

-   **“docker ps”** this shows all running process managed by docker

<!-- -->

    sudo docker stop $CONTAINER_ID

Stop the container, since we don’t need it anymore.

-   **“docker stop”** This stops a container
-   **\$CONTAINER\_ID** The Id of the container we want to stop.

<!-- -->

    sudo docker ps

Make sure it is really stopped.

**Video:**

See the example in action

The next example in the series is a [*Node.js Web
App*](../nodejs_web_app/#nodejs-web-app) example, or you could skip to
any of the other examples:

-   [*Node.js Web App*](../nodejs_web_app/#nodejs-web-app)
-   [*Redis Service*](../running_redis_service/#running-redis-service)
-   [*SSH Daemon Service*](../running_ssh_service/#running-ssh-service)
-   [*CouchDB
    Service*](../couchdb_data_volumes/#running-couchdb-service)
-   [*PostgreSQL Service*](../postgresql_service/#postgresql-service)
-   [*Building an Image with MongoDB*](../mongodb/#mongodb-image)
-   [*Python Web App*](../python_web_app/#python-web-app)


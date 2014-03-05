Python Web App[¶](#python-web-app "Permalink to this headline")
===============================================================

Note

-   This example assumes you have Docker running in daemon mode. For
    more information please see [*Check your Docker
    install*](../hello_world/#running-examples).
-   **If you don’t like sudo** then see [*Giving non-root
    access*](../../installation/binaries/#dockergroup)

While using Dockerfiles is the preferred way to create maintainable and
repeatable images, its useful to know how you can try things out and
then commit your live changes to an image.

The goal of this example is to show you how you can modify your own
Docker images by making changes to a running container, and then saving
the results as a new image. We will do that by making a simple ‘hello
world’ Flask web application image.

Download the initial image[¶](#download-the-initial-image "Permalink to this headline")
---------------------------------------------------------------------------------------

Download the `shykes/pybuilder`{.docutils .literal} Docker image from
the `http://index.docker.io`{.docutils .literal} registry.

This image contains a `buildapp`{.docutils .literal} script to download
the web app and then `pip install`{.docutils .literal} any required
modules, and a `runapp`{.docutils .literal} script that finds the
`app.py`{.docutils .literal} and runs it.

    $ sudo docker pull shykes/pybuilder

Note

This container was built with a very old version of docker (May 2013 -
see [shykes/pybuilder](https://github.com/shykes/pybuilder) ), when the
`Dockerfile`{.docutils .literal} format was different, but the image can
still be used now.

Interactively make some modifications[¶](#interactively-make-some-modifications "Permalink to this headline")
-------------------------------------------------------------------------------------------------------------

We then start a new container running interactively using the image.
First, we set a `URL`{.docutils .literal} variable that points to a
tarball of a simple helloflask web app, and then we run a command
contained in the image called `buildapp`{.docutils .literal}, passing it
the `$URL`{.docutils .literal} variable. The container is given a name
`pybuilder_run`{.docutils .literal} which we will use in the next steps.

While this example is simple, you could run any number of interactive
commands, try things out, and then exit when you’re done.

    $ sudo docker run -i -t -name pybuilder_run shykes/pybuilder bash

    $$ URL=http://github.com/shykes/helloflask/archive/master.tar.gz
    $$ /usr/local/bin/buildapp $URL
    [...]
    $$ exit

Commit the container to create a new image[¶](#commit-the-container-to-create-a-new-image "Permalink to this headline")
-----------------------------------------------------------------------------------------------------------------------

Save the changes we just made in the container to a new image called
`/builds/github.com/shykes/helloflask/master`{.docutils .literal}. You
now have 3 different ways to refer to the container: name
`pybuilder_run`{.docutils .literal}, short-id `c8b2e8228f11`{.docutils
.literal}, or long-id
`c8b2e8228f11b8b3e492cbf9a49923ae66496230056d61e07880dc74c5f495f9`{.docutils
.literal}.

    $ sudo docker commit pybuilder_run /builds/github.com/shykes/helloflask/master
    c8b2e8228f11b8b3e492cbf9a49923ae66496230056d61e07880dc74c5f495f9

Run the new image to start the web worker[¶](#run-the-new-image-to-start-the-web-worker "Permalink to this headline")
---------------------------------------------------------------------------------------------------------------------

Use the new image to create a new container with network port 5000
mapped to a local port

    $ sudo docker run -d -p 5000 --name web_worker /builds/github.com/shykes/helloflask/master /usr/local/bin/runapp

-   **“docker run -d “** run a command in a new container. We pass “-d”
    so it runs as a daemon.
-   **“-p 5000”** the web app is going to listen on this port, so it
    must be mapped from the container to the host system.
-   **/usr/local/bin/runapp** is the command which starts the web app.

View the container logs[¶](#view-the-container-logs "Permalink to this headline")
---------------------------------------------------------------------------------

View the logs for the new `web_worker`{.docutils .literal} container and
if everything worked as planned you should see the line
`Running on http://0.0.0.0:5000/`{.docutils .literal} in the log output.

To exit the view without stopping the container, hit Ctrl-C, or open
another terminal and continue with the example while watching the result
in the logs.

    $ sudo docker logs -f web_worker
     * Running on http://0.0.0.0:5000/

See the webapp output[¶](#see-the-webapp-output "Permalink to this headline")
-----------------------------------------------------------------------------

Look up the public-facing port which is NAT-ed. Find the private port
used by the container and store it inside of the `WEB_PORT`{.docutils
.literal} variable.

Access the web app using the `curl`{.docutils .literal} binary. If
everything worked as planned you should see the line
`Hello world!`{.docutils .literal} inside of your console.

    $ WEB_PORT=$(sudo docker port web_worker 5000 | awk -F: '{ print $2 }')

    # install curl if necessary, then ...
    $ curl http://127.0.0.1:$WEB_PORT
    Hello world!

Clean up example containers and images[¶](#clean-up-example-containers-and-images "Permalink to this headline")
---------------------------------------------------------------------------------------------------------------

    $ sudo docker ps --all

List `--all`{.docutils .literal} the Docker containers. If this
container had already finished running, it will still be listed here
with a status of ‘Exit 0’.

    $ sudo docker stop web_worker
    $ sudo docker rm web_worker pybuilder_run
    $ sudo docker rmi /builds/github.com/shykes/helloflask/master shykes/pybuilder:latest

And now stop the running web worker, and delete the containers, so that
we can then delete the images that we used.

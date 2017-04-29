Use `docker image push` to share your images to the [Docker Hub](https://hub.docker.com)
registry or to a self-hosted one.

Refer to **docker-image-tag(1)** for more information about valid image and tag names.

Killing the **docker image push** process, for example by pressing **CTRL-c** while it
is running in a terminal, terminates the push operation.

Registry credentials are managed by **docker-login(1)**.

# EXAMPLES

## Pushing a new image to a registry

First save the new image by finding the container ID (using **docker container ls**)
and then committing it to a new image name.  Note that only a-z0-9-_. are
allowed when naming images:

    # docker container commit c16378f943fe rhel-httpd

Now, push the image to the registry using the image ID. In this example the
registry is on host named `registry-host` and listening on port `5000`. To do
this, tag the image with the host name or IP address, and the port of the
registry:

    # docker image tag rhel-httpd registry-host:5000/myadmin/rhel-httpd
    # docker image push registry-host:5000/myadmin/rhel-httpd

Check that this worked by running:

    # docker image ls

You should see both `rhel-httpd` and `registry-host:5000/myadmin/rhel-httpd`
listed.

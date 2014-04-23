% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-push - Push an image or a repository to the registry

# SYNOPSIS
**docker push** NAME[:TAG]

# DESCRIPTION
Push an image or a repository to a registry. The default registry is the Docker 
Index located at [index.docker.io](https://index.docker.io/v1/). However the 
image can be pushed to another, perhaps private, registry as demonstrated in 
the example below.

# EXAMPLE

# Pushing a new image to a registry

First save the new image by finding the container ID (using **docker ps**)
and then committing it to a new image name:

    # docker commit c16378f943fe rhel-httpd

Now push the image to the registry using the image ID. In this example
the registry is on host named registry-host and listening on port 5000.
Default Docker commands will push to the default `index.docker.io`
registry. Instead, push to the local registry, which is on a host called
registry-host*. To do this, tag the image with the host name or IP
address, and the port of the registry:

    # docker tag rhel-httpd registry-host:5000/myadmin/rhel-httpd
    # docker push registry-host:5000/myadmin/rhel-httpd

Check that this worked by running:

    # docker images

You should see both `rhel-httpd` and `registry-host:5000/myadmin/rhel-httpd`
listed.

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.

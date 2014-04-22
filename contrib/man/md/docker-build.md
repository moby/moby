% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-build - Build a container image from a Dockerfile source at PATH

# SYNOPSIS
**docker build** [**--no-cache**[=*false*] [**-q**|**--quiet**[=*false*]
 [**-rm**] [**-t**|**--tag**=*tag*] PATH | URL | -

# DESCRIPTION
This will read the Dockerfile from the directory specified in **PATH**.
It also sends any other files and directories found in the current
directory to the Docker daemon. The contents of this directory would
be used by **ADD** commands found within the Dockerfile.

Warning, this will send a lot of data to the Docker daemon depending
on the contents of the current directory. The build is run by the Docker 
daemon, not by the CLI, so the whole context must be transferred to the daemon. 
The Docker CLI reports "Uploading context" when the context is sent to 
the daemon.

When a single Dockerfile is given as the URL, then no context is set.
When a Git repository is set as the **URL**, the repository is used
as context.

# OPTIONS

**-q**, **--quiet**=*true*|*false*
   When set to true, suppress verbose build output. Default is *false*.

**--rm**=*true*|*false*
   When true, remove intermediate containers that are created during the
build process. The default is true.

**-t**, **--tag**=*tag*
   Tag to be applied to the resulting image on successful completion of
the build.

**--no-cache**=*true*|*false*
   When set to true, do not use a cache when building the image. The
default is *false*.

# EXAMPLES

## Building an image using a Dockefile located inside the current directory

Docker images can be built using the build command and a Dockerfile:

    docker build .

During the build process Docker creates intermediate images. In order to
keep them, you must explicitly set `--rm=false`.

    docker build --rm=false .

A good practice is to make a sub-directory with a related name and create
the Dockerfile in that directory. For example, a directory called mongo may
contain a Dockerfile to create a Docker MongoDB image. Likewise, another
directory called httpd may be used to store Dockerfiles for Apache web
server images.

It is also a good practice to add the files required for the image to the
sub-directory. These files will then be specified with the `ADD` instruction
in the Dockerfile. Note: If you include a tar file (a good practice!), then
Docker will automatically extract the contents of the tar file
specified within the `ADD` instruction into the specified target.

## Building an image using a URL

This will clone the specified Github repository from the URL and use it
as context. The Dockerfile at the root of the repository is used as
Dockerfile. This only works if the Github repository is a dedicated
repository.

    docker build github.com/scollier/Fedora-Dockerfiles/tree/master/apache

Note: You can set an arbitrary Git repository via the `git://` schema.

# HISTORY
March 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.

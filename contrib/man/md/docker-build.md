% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-build - Build a container image from a Dockerfile source at PATH

# SYNOPSIS
**docker build** [**--no-cache**[=*false*] [**-q**|**--quiet**[=*false*]
 [**-rm**] [**-t**|**--tag**=*tag*] PATH | URL | -

# DESCRIPTION
This will read the Dockerfile from the directory specified in **PATH**. It also
 sends any other files and directories found in the current directory to the
Docker daemon. The contents of this directory would be used by **ADD** commands
found within the Dockerfile. Warning, this will send a lot of data to the Docker
daemon if the current directory contains a lot of data.

If the absolute path is provided instead of ‘.’, only the files and directories
required by the **ADD** command from the Dockerfile will be added to the context
 and transferred to the Docker daemon.

When a single Dockerfile is given as URL, then no context is set. When a Git
repository is set as **URL**, the repository is used as context.

# OPTIONS

**-q**, **--quiet**=*true*|*false*
:  When set to true, suppress verbose build output. Default is *false*.

**--rm**=*true*|*false*
:  When true, remove intermediate containers that are created during the build process. The default is true.

**-t**, **--tag**=*tag*
:  Tag to be applied to the resulting image on successful completion of the build.

**--no-cache**=*true*|*false*
:  When set to true, do not use a cache when building the image. The default is *false*.

# EXAMPLES

## Building an image from current directory

Using a Dockerfile, Docker images are built using the build command:

    docker build .

If, for some reason, you do not what to remove the intermediate containers created during the build you must set --rm=false:

    docker build --rm=false .

A good practice is to make a sub-directory with a related name and create the
Dockerfile in that directory. E.g. a directory called mongo may contain a
Dockerfile for a MongoDB image, or a directory called httpd may contain a
Dockerfile for an Apache web server.

It is also good practice to add the files required for the image to the
sub-directory. These files will be then specified with the `ADD` instruction in
the Dockerfile. Note: if you include a tar file, which is good practice, then
Docker will automatically extract the contents of the tar file specified in the
`ADD` instruction into the specified target.

## Building an image using a URL

This will clone the Github repository and use it as context. The Dockerfile at the root of the repository is used as Dockerfile. This only works if the Github repository is a dedicated repository.

    docker build github.com/scollier/Fedora-Dockerfiles/tree/master/apache

Note that you can specify an arbitrary Git repository by using the `git://`
schema.

# HISTORY
March 2014, Originally compiled by William Henry (whenry at redhat dot com) based on docker.io source material and internal work.

% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-commit - Create a new image from a container's changes

# SYNOPSIS
**docker commit**
[**-a**|**--author**[=*AUTHOR*]]
[**--help**]
[**-c**|**--change**[= []**]]
[**-m**|**--message**[=*MESSAGE*]]
[**-p**|**--pause**[=*true*]]
CONTAINER [REPOSITORY[:TAG]]

# DESCRIPTION
Using an existing container's name or ID you can create a new image.

# OPTIONS
**-a**, **--author**=""
   Author (e.g., "John Hannibal Smith <hannibal@a-team.com>")

**-c** , **--change**=[]
   Apply specified Dockerfile instructions while committing the image
   Supported Dockerfile instructions: `CMD`|`ENTRYPOINT`|`ENV`|`EXPOSE`|`ONBUILD`|`USER`|`VOLUME`|`WORKDIR`

**--help**
  Print usage statement

**-m**, **--message**=""
   Commit message

**-p**, **--pause**=*true*|*false*
   Pause container during commit. The default is *true*.

# EXAMPLES

## Creating a new image from an existing container
An existing Fedora based container has had Apache installed while running
in interactive mode with the bash shell. Apache is also running. To
create a new image run `docker ps` to find the container's ID and then run:

    # docker commit -m="Added Apache to Fedora base image" \
      -a="A D Ministrator" 98bd7fc99854 fedora/fedora_httpd:20

## Apply specified Dockerfile instructions while committing the image
If an existing container was created without the DEBUG environment
variable set to "true", you can create a new image based on that
container by first getting the container's ID with `docker ps` and
then running:

    # docker commit -c="ENV DEBUG true" 98bd7fc99854 debug-image

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and in
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
July 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
Oct 2014, updated by Daniel, Dao Quang Minh <daniel at nitrous dot io>

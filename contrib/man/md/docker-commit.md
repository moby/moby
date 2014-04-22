% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-commit - Create a new image from the changes to an existing
container

# SYNOPSIS
**docker commit** **-a**|**--author**[=""] **-m**|**--message**[=""]
CONTAINER [REPOSITORY[:TAG]]

# DESCRIPTION
Using an existing container's name or ID you can create a new image.

# OPTIONS
**-a, --author**=""
   Author name. (eg. "John Hannibal Smith <hannibal@a-team.com>"

**-m, --message**=""
   Commit message

# EXAMPLES

## Creating a new image from an existing container
An existing Fedora based container has had Apache installed while running
in interactive mode with the bash shell. Apache is also running. To
create a new image run docker ps to find the container's ID and then run:

    # docker commit -me= "Added Apache to Fedora base image" \
      --a="A D Ministrator" 98bd7fc99854 fedora/fedora_httpd:20

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and in
% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-history - Show the history of an image

# SYNOPSIS
**docker history** **--no-trunc**[=*false*] [**-q**|**--quiet**[=*false*]]
 IMAGE

# DESCRIPTION

Show the history of when and how an image was created.

# OPTIONS

**--no-trunc**=*true*|*false*
   When true don't truncate output. Default is false

**-q**, **--quiet=*true*|*false*
   When true only show numeric IDs. Default is false.

# EXAMPLE
    $ sudo docker history fedora
    IMAGE          CREATED          CREATED BY                                      SIZE
    105182bb5e8b   5 days ago       /bin/sh -c #(nop) ADD file:71356d2ad59aa3119d   372.7 MB
    73bd853d2ea5   13 days ago      /bin/sh -c #(nop) MAINTAINER Lokesh Mandvekar   0 B
    511136ea3c5a   10 months ago                                                    0 B

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.

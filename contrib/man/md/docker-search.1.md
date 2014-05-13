% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-search - Search the docker index for images

# SYNOPSIS
**docker search** **--no-trunc**[=*false*] **-t**|**--trusted**[=*false*]
 **-s**|**--stars**[=*0*] TERM

# DESCRIPTION

Search an index for an image with that matches the term TERM. The table
of images returned displays the name, description (truncated by default),
number of stars awarded, whether the image is official, and whether it
is trusted.

# OPTIONS
**--no-trunc**=*true*|*false*
   When true display the complete description. The default is false.

**-s**, **--stars**=NUM
   Only displays with at least NUM (integer) stars. I.e. only those images
ranked >=NUM.

**-t**, **--trusted**=*true*|*false*
   When true only show trusted builds. The default is false.

# EXAMPLE

## Search the registry for ranked images

Search the registry for the term 'fedora' and only display those images
ranked 3 or higher:

    $ sudo docker search -s 3 fedora
    NAME                  DESCRIPTION                                    STARS OFFICIAL  TRUSTED
    mattdm/fedora         A basic Fedora image corresponding roughly...  50
    fedora                (Semi) Official Fedora base image.             38
    mattdm/fedora-small   A small Fedora image on which to build. Co...  8
    goldmann/wildfly      A WildFly application server running on a ...  3               [OK]

## Search the registry for trusted images

Search the registry for the term 'fedora' and only display trusted images
ranked 1 or higher:

    $ sudo docker search -s 1 -t fedora
    NAME               DESCRIPTION                                     STARS OFFICIAL  TRUSTED
    goldmann/wildfly   A WildFly application server running on a ...   3               [OK]
    tutum/fedora-20    Fedora 20 image with SSH access. For the r...   1               [OK]

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.

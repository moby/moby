% DOCKER(1) Docker User Manuals
% Docker Community
% November 2014
# NAME
docker-volumes-ls - List volumes

# SYNOPSIS
**docker volumes ls**
[**-f**|**--filter**[=*[]*]]
[**-q**|**--quiet**[=*false*]]
[**-s**|**--size**[=*false*]]
 [NAME]

# DESCRIPTION
This command lists the volumes registered with the local Docker instance.

# OPTIONS
**-f**, **--filter**=[]
   Provide filter values (i.e. 'dangling=true')

**-q**, **--quiet**=*true*|*false*
   Only show numeric names. The default is *false*.

**-s**, **--size**=*true*|*false*
   Show the size of the volume on disk.

# EXAMPLES

## Listing the volumes

To list the registered volumes:

    docker volumes ls

The list will contain the volume name, creation date, and the number of
containers using the volume. Columns: NAME CREATED USED COUNT. The **ls**
subcommand is optional and instead be run as just **docker volumes**, without
the **ls**.

To get a list of volumes with the size of the volume on disk, use **-s**:

    docker volumes ls -s

## Listing only the volume names

Listing just the volume names. This can be useful for some automated
tools.

    docker volumes ls -q

# HISTORY
November 2014, updated by Brian Goff <cpuguy83@mail.com>

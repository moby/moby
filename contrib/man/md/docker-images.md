% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker-images - List the images in the local repository

# SYNOPSIS
**docker images**
[**-a**|**--all**=*false*]
[**--no-trunc**[=*false*]
[**-q**|**--quiet**[=*false*]
[**-t**|**--tree**=*false*]
[**-v**|**--viz**=*false*]
[NAME]

# DESCRIPTION
This command lists the images stored in the local Docker repository.

By default, intermediate images, used during builds, are not listed. Some of the
output, e.g. image ID, is truncated, for space reasons. However the truncated
image ID, and often the first few characters, are enough to be used in other
Docker commands that use the image ID. The output includes repository, tag, image
ID, date created and the virtual size.

The title REPOSITORY for the first title may seem confusing. It is essentially
the image name. However, because you can tag a specific image, and multiple tags
(image instances) can be associated with a single name, the name is really a
repository for all tagged images of the same name. For example consider an image
called fedora. It may be tagged with 18, 19, or 20, etc. to manage different
versions.

# OPTIONS

**-a**, **--all**=*true*|*false*
   When set to true, also include all intermediate images in the list. The
default is false.

**--no-trunc**=*true*|*false*
   When set to true, list the full image ID and not the truncated ID. The
default is false.

**-q**, **--quiet**=*true*|*false*
   When set to true, list the complete image ID as part of the output. The
default is false.

**-t**, **--tree**=*true*|*false*
   When set to true, list the images in a tree dependency tree (hierarchy)
format. The default is false.

**-v**, **--viz**=*true*|*false*
   When set to true, list the graph in graphviz format. The default is
*false*.

# EXAMPLES

## Listing the images

To list the images in a local repository (not the registry) run:

    docker images

The list will contain the image repository name, a tag for the image, and an
image ID, when it was created and its virtual size. Columns: REPOSITORY, TAG,
IMAGE ID, CREATED, and VIRTUAL SIZE.

To get a verbose list of images which contains all the intermediate images
used in builds use **-a**:

    docker images -a

## List images dependency tree hierarchy

To list the images in the local repository (not the registry) in a dependency
tree format, use the **-t** option.

    docker images -t

This displays a staggered hierarchy tree where the less indented image is
the oldest with dependent image layers branching inward (to the right) on
subsequent lines. The newest or top level image layer is listed last in
any tree branch.

## List images in GraphViz format

To display the list in a format consumable by a GraphViz tools run with
**-v**. For example to produce a .png graph file of the hierarchy use:

    docker images --viz | dot -Tpng -o docker.png

## Listing only the shortened image IDs

Listing just the shortened image IDs. This can be useful for some automated
tools.

    docker images -q

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.io source material and internal work.

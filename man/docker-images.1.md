% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-images - List images

# SYNOPSIS
**docker images**
[**--help**]
[**-a**|**--all**]
[**--digests**]
[**-f**|**--filter**[=*[]*]]
[**--format**=*"TEMPLATE"*]
[**--no-trunc**]
[**-q**|**--quiet**]
[REPOSITORY[:TAG]]

# DESCRIPTION
This command lists the images stored in the local Docker repository.

By default, intermediate images, used during builds, are not listed. Some of the
output, e.g., image ID, is truncated, for space reasons. However the truncated
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
   Show all images (by default filter out the intermediate image layers). The default is *false*.

**--digests**=*true*|*false*
   Show image digests. The default is *false*.

**-f**, **--filter**=[]
   Filters the output based on these conditions:
   - dangling=(true|false) - find unused images
   - label=<key> or label=<key>=<value>
   - before=(<image-name>[:tag]|<image-id>|<image@digest>)
   - since=(<image-name>[:tag]|<image-id>|<image@digest>)

**--format**="*TEMPLATE*"
   Pretty-print images using a Go template.
   Valid placeholders:
      .ID - Image ID
      .Repository - Image repository
      .Tag - Image tag
      .Digest - Image digest
      .CreatedSince - Elapsed time since the image was created
      .CreatedAt - Time when the image was created
      .Size - Image disk size

**--help**
  Print usage statement

**--no-trunc**=*true*|*false*
   Don't truncate output. The default is *false*.

**-q**, **--quiet**=*true*|*false*
   Only show numeric IDs. The default is *false*.

# EXAMPLES

## Listing the images

To list the images in a local repository (not the registry) run:

    docker images

The list will contain the image repository name, a tag for the image, and an
image ID, when it was created and its virtual size. Columns: REPOSITORY, TAG,
IMAGE ID, CREATED, and SIZE.

The `docker images` command takes an optional `[REPOSITORY[:TAG]]` argument
that restricts the list to images that match the argument. If you specify
`REPOSITORY`but no `TAG`, the `docker images` command lists all images in the
given repository.

    docker images java

The `[REPOSITORY[:TAG]]` value must be an "exact match". This means that, for example,
`docker images jav` does not match the image `java`.

If both `REPOSITORY` and `TAG` are provided, only images matching that
repository and tag are listed.  To find all local images in the "java"
repository with tag "8" you can use:

    docker images java:8

To get a verbose list of images which contains all the intermediate images
used in builds use **-a**:

    docker images -a

Previously, the docker images command supported the --tree and --dot arguments,
which displayed different visualizations of the image data. Docker core removed
this functionality in the 1.7 version. If you liked this functionality, you can
still find it in the third-party dockviz tool: https://github.com/justone/dockviz.

## Listing only the shortened image IDs

Listing just the shortened image IDs. This can be useful for some automated
tools.

    docker images -q

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>

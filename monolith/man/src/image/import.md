Create a new filesystem image from the contents of a tarball (`.tar`,
`.tar.gz`, `.tgz`, `.bzip`, `.tar.xz`, `.txz`) into it, then optionally tag it.


# EXAMPLES

## Import from a remote location

    # docker image import http://example.com/exampleimage.tgz example/imagerepo

## Import from a local file

Import to docker via pipe and stdin:

    # cat exampleimage.tgz | docker image import - example/imagelocal

Import with a commit message. 

    # cat exampleimage.tgz | docker image import --message "New image imported from tarball" - exampleimagelocal:new

Import to a Docker image from a local file.

    # docker image import /path/to/exampleimage.tgz 


## Import from a local file and tag

Import to docker via pipe and stdin:

    # cat exampleimageV2.tgz | docker image import - example/imagelocal:V-2.0

## Import from a local directory

    # tar -c . | docker image import - exampleimagedir

## Apply specified Dockerfile instructions while importing the image
This example sets the docker image ENV variable DEBUG to true by default.

    # tar -c . | docker image import -c="ENV DEBUG true" - exampleimagedir

# See also
**docker-export(1)** to export the contents of a filesystem as a tar archive to STDOUT.

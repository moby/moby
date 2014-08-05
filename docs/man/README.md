Docker Documentation
====================

This directory contains the Docker user manual in the Markdown format.
Do *not* edit the man pages in the man1 directory. Instead, amend the
Markdown (*.md) files.

# File List

    docker.md
    docker-attach.md
    docker-build.md
    docker-commit.md
    docker-cp.md
    docker-diff.md
    docker-events.md
    docker-export.md
    docker-history.md
    docker-images.md
    docker-import.md
    docker-info.md
    docker-inspect.md
    docker-kill.md
    docker-load.md
    docker-login.md
    docker-logs.md
    docker-port.md
    docker-ps.md
    docker-pull.md
    docker-push.md
    docker-restart.md
    docker-rmi.md
    docker-rm.md
    docker-run.md
    docker-save.md
    docker-search.md
    docker-start.md
    docker-stop.md
    docker-tag.md
    docker-top.md
    docker-wait.md
    Dockerfile
    md2man-all.sh

# Generating man pages from the Markdown files

The recommended approach for generating the man pages is via a Docker
container using the supplied `Dockerfile` to create an image with the correct
environment. This uses `go-md2man`, a pure Go Markdown to man page generator.

## Building the md2man image

There is a `Dockerfile` provided in the `docker/docs/man` directory.

Using this `Dockerfile`, create a Docker image tagged `docker/md2man`:

    docker build -t docker/md2man .

## Utilizing the image

Once the image is built, run a container using the image with *volumes*:

    docker run -v /<path-to-git-dir>/docker/docs/man:/docs:rw \
    -w /docs -i docker/md2man /docs/md2man-all.sh

The `md2man` Docker container will process the Markdown files and generate
the man pages inside the `docker/docs/man/man1` directory using
Docker volumes. For more information on Docker volumes see the man page for
`docker run` and also look at the article [Sharing Directories via Volumes]
(http://docs.docker.com/use/working_with_volumes/).

Docker Documentation
====================

This directory contains the Docker user manual in the Markdown format.
Do *not* edit the man pages in the man1 directory. Instead, amend the
Markdown (*.md) files.

# File List

    docker-attach.1.md
    docker-build.1.md
    docker-commit.1.md
    docker-cp.1.md
    docker-diff.1.md
    docker-events.1.md
    docker-exec.1.md
    docker-export.1.md
    docker-history.1.md
    docker-images.1.md
    docker-import.1.md
    docker-info.1.md
    docker-inspect.1.md
    docker-kill.1.md
    docker-load.1.md
    docker-login.1.md
    docker-logout.1.md
    docker-logs.1.md
    docker-pause.1.md
    docker-port.1.md
    docker-ps.1.md
    docker-pull.1.md
    docker-push.1.md
    docker-restart.1.md
    docker-rm.1.md
    docker-rmi.1.md
    docker-run.1.md
    docker-save.1.md
    docker-search.1.md
    docker-start.1.md
    docker-stop.1.md
    docker-tag.1.md
    docker-top.1.md
    docker-unpause.1.md
    docker-version.1.md
    docker-wait.1.md
    docker.1.md
    Dockerfile
    Dockerfile.5.md
    md2man-all.sh
    README.md

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

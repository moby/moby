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

The recommended approach for generating the man pages is via a  Docker 
container. Using the supplied Dockerfile, Docker will create a Fedora based 
container and isolate the Pandoc installation. This is a seamless process, 
saving you from dealing with Pandoc and dependencies on your own computer.

## Building the Fedora / Pandoc image

There is a Dockerfile provided in the `docker/contrib/man/md` directory.

Using this Dockerfile, create a Docker image tagged `fedora/pandoc`:

    docker build  -t fedora/pandoc .

## Utilizing the Fedora / Pandoc image

Once the image is built, run a container using the image with *volumes*:

    docker run -v /<path-to-git-dir>/docker/contrib/man:/pandoc:rw \
    -w /pandoc -i fedora/pandoc /pandoc/md/md2man-all.sh

The Pandoc Docker container will process the Markdown files and generate
the man pages inside the `docker/contrib/man/man1` directory using
Docker volumes. For more information on Docker volumes see the man page for
`docker run` and also look at the article [Sharing Directories via Volumes]
(http://docs.docker.io/use/working_with_volumes/).

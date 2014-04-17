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

There are three ways to generate the man pages:
* Each page manually
* All pages manually
* Via a *Pandoc* Docker container (**Recommended**)

The first and second approach require you to install the Pandoc package
 on your computer using the default package installer for the system.
You should check if Pandoc is available before trying to do so.

The recommended approach is the Pandoc Docker container one.
Using the supplied Dockerfile, Docker creates a Fedora based container
and isolates the Pandoc installation.
This is a seamless process, saving you from dealing with Pandoc and
dependencies on your own computer.

## Each page manually

You can generate the man pages with:

    pandoc -s -t man docker-<command>.md ../man1/docker-<command>.1

The results will be stored ../man1

## All pages manually

You can generate *all* the man pages from the source using:

    for FILE in *.md
    do
    pandoc -s -t man $FILE -o ../man1/"${FILE%.*}".1
    done

## Using the pandoc Container

There is a Dockerfile provided in the `docker/contrib/man/md` directory.

Using this Dockerfile, create a Docker image tagged `fedora/pandoc`.

    docker build  -t fedora/pandoc .

Once the image is built, create a container inside the
`docker/contrib/man/md` directory using the it:

    docker run -v /<path-to-git-dir>/docker/contrib/man:/pandoc:rw \
    -w /pandoc -i fedora/pandoc /pandoc/md/md2man-all.sh

The Pandoc Docker container will process the Markdown files and generate
 the man pages inside the `docker/contrib/man/man1` directory using
 Docker volumes.

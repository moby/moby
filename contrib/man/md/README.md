Docker Documentation
====================

This directory contains the docker user manual in Markdown format.
DO NOT edit the man pages in the man1 directory. Instead make changes here.

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

# Generating man pages from the Markdown

There are three ways to generate the man pages:
* Manually Individually
* Using the Script
* Using a the Pandoc Container (**Recommended**)

The first and second approach require you to install pandoc packages
 on your host using the host operating systems package installer. Check
to see if pandoc is available if you choose that method.

The Pandoc container approach is recommneded because the conversion process
is isolated inside a fedora container and thereofre does not require you
find and install pandoc on your host.

## Manually Individually

You can generate the manpage by:

    pandoc -s -t man docker-<command>.md ../man1/docker-<command>.1

The resulting man pages are stored in ../man1

## Manually All

Or regenerate all the manpages from this source using:

    for FILE in *.md
    do
    pandoc -s -t man $FILE -o ../man1/"${FILE%.*}".1
    done

## Using the pandoc Container

There is a Dockerfile provided in the `docker/contrib/man/md` directory.

Use this Dockerfile to create a `fedora/pandoc` container:

    # docker build  -t fedora/pandoc .

After the container is created run the following command from your
`docker/contrib/man/md` directory:

    # docker run -v /<path-to-git-dir>/docker/contrib/man:/pandoc:rw \
    -w /pandoc -i fedora/pandoc /pandoc/md/md2man-all.sh

This will generate all man files into `docker/contrib/man/man1`.

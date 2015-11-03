<!--[metadata]>
+++
title = "Best practices for writing Dockerfiles"
description = "Hints, tips and guidelines for writing clean, reliable Dockerfiles"
keywords = ["Examples, Usage, base image, docker, documentation, dockerfile, best practices, hub,  official repo"]
[menu.main]
parent = "smn_images"
+++
<![end-metadata]-->

# Best practices for writing Dockerfiles

## Overview

Docker can build images automatically by reading the instructions from a
`Dockerfile`, a text file that contains all the commands, in order, needed to
build a given image. `Dockerfile`s adhere to a specific format and use a
specific set of instructions. You can learn the basics on the
[Dockerfile Reference](../reference/builder.md) page. If
you’re new to writing `Dockerfile`s, you should start there.

This document covers the best practices and methods recommended by Docker,
Inc. and the Docker community for creating easy-to-use, effective
`Dockerfile`s. We strongly suggest you follow these recommendations (in fact,
if you’re creating an Official Image, you *must* adhere to these practices).

You can see many of these practices and recommendations in action in the [buildpack-deps `Dockerfile`](https://github.com/docker-library/buildpack-deps/blob/master/jessie/Dockerfile).

> Note: for more detailed explanations of any of the Dockerfile commands
>mentioned here, visit the [Dockerfile Reference](../reference/builder.md) page.

## General guidelines and recommendations

### Containers should be ephemeral

The container produced by the image your `Dockerfile` defines should be as
ephemeral as possible. By “ephemeral,” we mean that it can be stopped and
destroyed and a new one built and put in place with an absolute minimum of
set-up and configuration.

### Use a .dockerignore file

In most cases, it's best to put each Dockerfile in an empty directory. Then,
add to that directory only the files needed for building the Dockerfile. To
increase the build's performance, you can exclude files and directories by
adding a `.dockerignore` file to that directory as well. This file supports
exclusion patterns similar to `.gitignore` files. For information on creating one,
see the [.dockerignore file](../reference/builder.md#dockerignore-file).

### Avoid installing unnecessary packages

In order to reduce complexity, dependencies, file sizes, and build times, you
should avoid installing extra or unnecessary packages just because they
might be “nice to have.” For example, you don’t need to include a text editor
in a database image.

### Run only one process per container

In almost all cases, you should only run a single process in a single
container. Decoupling applications into multiple containers makes it much
easier to scale horizontally and reuse containers. If that service depends on
another service, make use of [container linking](../userguide/networking/default_network/dockerlinks.md).

### Minimize the number of layers

You need to find the balance between readability (and thus long-term
maintainability) of the `Dockerfile` and minimizing the number of layers it
uses. Be strategic and cautious about the number of layers you use.

### Sort multi-line arguments

Whenever possible, ease later changes by sorting multi-line arguments
alphanumerically. This will help you avoid duplication of packages and make the
list much easier to update. This also makes PRs a lot easier to read and
review. Adding a space before a backslash (`\`) helps as well.

Here’s an example from the [`buildpack-deps` image](https://github.com/docker-library/buildpack-deps):

    RUN apt-get update && apt-get install -y \
      bzr \
      cvs \
      git \
      mercurial \
      subversion

### Build cache

During the process of building an image Docker will step through the
instructions in your `Dockerfile` executing each in the order specified.
As each instruction is examined Docker will look for an existing image in its
cache that it can reuse, rather than creating a new (duplicate) image.
If you do not want to use the cache at all you can use the ` --no-cache=true`
option on the `docker build` command.

However, if you do let Docker use its cache then it is very important to
understand when it will, and will not, find a matching image. The basic rules
that Docker will follow are outlined below:

* Starting with a base image that is already in the cache, the next
instruction is compared against all child images derived from that base
image to see if one of them was built using the exact same instruction. If
not, the cache is invalidated.

* In most cases simply comparing the instruction in the `Dockerfile` with one
of the child images is sufficient.  However, certain instructions require
a little more examination and explanation.

* For the `ADD` and `COPY` instructions, the contents of the file(s)
in the image are examined and a checksum is calculated for each file.
The last-modified and last-accessed times of the file(s) are not considered in
these checksums. During the cache lookup, the checksum is compared against the
checksum in the existing images. If anything has changed in the file(s), such
as the contents and metadata, then the cache is invalidated.

* Aside from the `ADD` and `COPY` commands cache checking will not look at the
files in the container to determine a cache match. For example, when processing
a `RUN apt-get -y update` command the files updated in the container
will not be examined to determine if a cache hit exists.  In that case just
the command string itself will be used to find a match.

Once the cache is invalidated, all subsequent `Dockerfile` commands will
generate new images and the cache will not be used.

## The Dockerfile instructions

Below you'll find recommendations for the best way to write the
various instructions available for use in a `Dockerfile`.

### FROM

[Dockerfile reference for the FROM instruction](../reference/builder.md#from)

Whenever possible, use current Official Repositories as the basis for your
image. We recommend the [Debian image](https://registry.hub.docker.com/_/debian/)
since it’s very tightly controlled and kept extremely minimal (currently under
100 mb), while still being a full distribution.

### RUN

[Dockerfile reference for the RUN instruction](../reference/builder.md#run)

As always, to make your `Dockerfile` more readable, understandable, and
maintainable, split long or complex `RUN` statements on multiple lines separated
with backslashes.

### apt-get

Probably the most common use-case for `RUN` is an application of `apt-get`. The
`RUN apt-get` command, because it installs packages, has several gotchas to look
out for.

You should avoid `RUN apt-get upgrade` or `dist-upgrade`, as many of the
“essential” packages from the base images won't upgrade inside an unprivileged
container. If a package contained in the base image is out-of-date, you should
contact its maintainers.
If you know there’s a particular package, `foo`, that needs to be updated, use
`apt-get install -y foo` to update automatically.

Always combine  `RUN apt-get update` with `apt-get install` in the same `RUN`
statement, for example:

        RUN apt-get update && apt-get install -y \
            package-bar \
            package-baz \
            package-foo


Using `apt-get update` alone in a `RUN` statement causes caching issues and
subsequent `apt-get install` instructions fail.
For example, say you have a Dockerfile:

        FROM ubuntu:14.04
        RUN apt-get update
        RUN apt-get install -y curl

After building the image, all layers are in the Docker cache. Suppose you later
modify `apt-get install` by adding extra package:

        FROM ubuntu:14.04
        RUN apt-get update
        RUN apt-get install -y curl nginx

Docker sees the initial and modified instructions as identical and reuses the
cache from previous steps. As a result the `apt-get update` is *NOT* executed
because the build uses the cached version. Because the `apt-get update` is not
run, your build can potentially get an outdated version of the `curl` and `nginx`
packages.

Using  `RUN apt-get update && apt-get install -y` ensures your Dockerfile
installs the latest package versions with no further coding or manual
intervention. This technique is known as "cache busting". You can also achieve
cache-busting by specifying a package version. This is known as version pinning,
for example:

        RUN apt-get update && apt-get install -y \
            package-bar \
            package-baz \
            package-foo=1.3.*

Version pinning forces the build to retrieve a particular version regardless of
what’s in the cache. This technique can also reduce failures due to unanticipated changes
in required packages.

Below is a well-formed `RUN` instruction that demonstrates all the `apt-get`
recommendations.

    RUN apt-get update && apt-get install -y \
        aufs-tools \
        automake \
        build-essential \
        curl \
        dpkg-sig \
        libcap-dev \
        libsqlite3-dev \
        lxc=1.0* \
        mercurial \
        reprepro \
        ruby1.9.1 \
        ruby1.9.1-dev \
        s3cmd=1.1.* \
     && apt-get clean \
     && rm -rf /var/lib/apt/lists/*

The `s3cmd` instructions specifies a version `1.1.0*`. If the image previously
used an older version, specifying the new one causes a cache bust of `apt-get
update` and ensure the installation of the new version. Listing packages on
each line can also prevent mistakes in package duplication.

In addition, cleaning up the apt cache and removing `/var/lib/apt/lists` helps
keep the image size down. Since the `RUN` statement starts with
`apt-get update`, the package cache will always be refreshed prior to
`apt-get install`.

### CMD

[Dockerfile reference for the CMD instruction](../reference/builder.md#cmd)

The `CMD` instruction should be used to run the software contained by your
image, along with any arguments. `CMD` should almost always be used in the
form of `CMD [“executable”, “param1”, “param2”…]`. Thus, if the image is for a
service (Apache, Rails, etc.), you would run something like
`CMD ["apache2","-DFOREGROUND"]`. Indeed, this form of the instruction is
recommended for any service-based image.

In most other cases, `CMD` should be given an interactive shell (bash, python,
perl, etc), for example, `CMD ["perl", "-de0"]`, `CMD ["python"]`, or
`CMD [“php”, “-a”]`. Using this form means that when you execute something like
`docker run -it python`, you’ll get dropped into a usable shell, ready to go.
`CMD` should rarely be used in the manner of `CMD [“param”, “param”]` in
conjunction with [`ENTRYPOINT`](../reference/builder.md#entrypoint), unless
you and your expected users are already quite familiar with how `ENTRYPOINT`
works.

### EXPOSE

[Dockerfile reference for the EXPOSE instruction](../reference/builder.md#expose)

The `EXPOSE` instruction indicates the ports on which a container will listen
for connections. Consequently, you should use the common, traditional port for
your application. For example, an image containing the Apache web server would
use `EXPOSE 80`, while an image containing MongoDB would use `EXPOSE 27017` and
so on.

For external access, your users can execute `docker run` with a flag indicating
how to map the specified port to the port of their choice.
For container linking, Docker provides environment variables for the path from
the recipient container back to the source (ie, `MYSQL_PORT_3306_TCP`).

### ENV

[Dockerfile reference for the ENV instruction](../reference/builder.md#env)

In order to make new software easier to run, you can use `ENV` to update the
`PATH` environment variable for the software your container installs. For
example, `ENV PATH /usr/local/nginx/bin:$PATH` will ensure that `CMD [“nginx”]`
just works.

The `ENV` instruction is also useful for providing required environment
variables specific to services you wish to containerize, such as Postgres’s
`PGDATA`.

Lastly, `ENV` can also be used to set commonly used version numbers so that
version bumps are easier to maintain, as seen in the following example:

    ENV PG_MAJOR 9.3
    ENV PG_VERSION 9.3.4
    RUN curl -SL http://example.com/postgres-$PG_VERSION.tar.xz | tar -xJC /usr/src/postgress && …
    ENV PATH /usr/local/postgres-$PG_MAJOR/bin:$PATH

Similar to having constant variables in a program (as opposed to hard-coding
values), this approach lets you change a single `ENV` instruction to
auto-magically bump the version of the software in your container.

### ADD or COPY

[Dockerfile reference for the ADD instruction](../reference/builder.md#add)<br/>
[Dockerfile reference for the COPY instruction](../reference/builder.md#copy)

Although `ADD` and `COPY` are functionally similar, generally speaking, `COPY`
is preferred. That’s because it’s more transparent than `ADD`. `COPY` only
supports the basic copying of local files into the container, while `ADD` has
some features (like local-only tar extraction and remote URL support) that are
not immediately obvious. Consequently, the best use for `ADD` is local tar file
auto-extraction into the image, as in `ADD rootfs.tar.xz /`.

If you have multiple `Dockerfile` steps that use different files from your
context, `COPY` them individually, rather than all at once. This will ensure that
each step's build cache is only invalidated (forcing the step to be re-run) if the
specifically required files change.

For example:

    COPY requirements.txt /tmp/
    RUN pip install /tmp/requirements.txt
    COPY . /tmp/

Results in fewer cache invalidations for the `RUN` step, than if you put the
`COPY . /tmp/` before it.

Because image size matters, using `ADD` to fetch packages from remote URLs is
strongly discouraged; you should use `curl` or `wget` instead. That way you can
delete the files you no longer need after they've been extracted and you won't
have to add another layer in your image. For example, you should avoid doing
things like:

    ADD http://example.com/big.tar.xz /usr/src/things/
    RUN tar -xJf /usr/src/things/big.tar.xz -C /usr/src/things
    RUN make -C /usr/src/things all

And instead, do something like:

    RUN mkdir -p /usr/src/things \
        && curl -SL http://example.com/big.tar.xz \
        | tar -xJC /usr/src/things \
        && make -C /usr/src/things all

For other items (files, directories) that do not require `ADD`’s tar
auto-extraction capability, you should always use `COPY`.

### ENTRYPOINT

[Dockerfile reference for the ENTRYPOINT instruction](../reference/builder.md#entrypoint)

The best use for `ENTRYPOINT` is to set the image's main command, allowing that
image to be run as though it was that command (and then use `CMD` as the
default flags).

Let's start with an example of an image for the command line tool `s3cmd`:

    ENTRYPOINT ["s3cmd"]
    CMD ["--help"]

Now the image can be run like this to show the command's help:

    $ docker run s3cmd

Or using the right parameters to execute a command:

    $ docker run s3cmd ls s3://mybucket

This is useful because the image name can double as a reference to the binary as
shown in the command above.

The `ENTRYPOINT` instruction can also be used in combination with a helper
script, allowing it to function in a similar way to the command above, even
when starting the tool may require more than one step.

For example, the [Postgres Official Image](https://registry.hub.docker.com/_/postgres/)
uses the following script as its `ENTRYPOINT`:

```bash
#!/bin/bash
set -e

if [ "$1" = 'postgres' ]; then
    chown -R postgres "$PGDATA"

    if [ -z "$(ls -A "$PGDATA")" ]; then
        gosu postgres initdb
    fi

    exec gosu postgres "$@"
fi

exec "$@"
```

> **Note**:
> This script uses [the `exec` Bash command](http://wiki.bash-hackers.org/commands/builtin/exec)
> so that the final running application becomes the container's PID 1. This allows
> the application to receive any Unix signals sent to the container.
> See the [`ENTRYPOINT`](../reference/builder.md#entrypoint)
> help for more details.


The helper script is copied into the container and run via `ENTRYPOINT` on
container start:

    COPY ./docker-entrypoint.sh /
    ENTRYPOINT ["/docker-entrypoint.sh"]

This script allows the user to interact with Postgres in several ways.

It can simply start Postgres:

    $ docker run postgres

Or, it can be used to run Postgres and pass parameters to the server:

    $ docker run postgres postgres --help

Lastly, it could also be used to start a totally different tool, such as Bash:

    $ docker run --rm -it postgres bash

### VOLUME

[Dockerfile reference for the VOLUME instruction](../reference/builder.md#volume)

The `VOLUME` instruction should be used to expose any database storage area,
configuration storage, or files/folders created by your docker container. You
are strongly encouraged to use `VOLUME` for any mutable and/or user-serviceable
parts of your image.

### USER

[Dockerfile reference for the USER instruction](../reference/builder.md#user)

If a service can run without privileges, use `USER` to change to a non-root
user. Start by creating the user and group in the `Dockerfile` with something
like `RUN groupadd -r postgres && useradd -r -g postgres postgres`.

> **Note:** Users and groups in an image get a non-deterministic
> UID/GID in that the “next” UID/GID gets assigned regardless of image
> rebuilds. So, if it’s critical, you should assign an explicit UID/GID.

You should avoid installing or using `sudo` since it has unpredictable TTY and
signal-forwarding behavior that can cause more problems than it solves. If
you absolutely need functionality similar to `sudo` (e.g., initializing the
daemon as root but running it as non-root), you may be able to use
[“gosu”](https://github.com/tianon/gosu).

Lastly, to reduce layers and complexity, avoid switching `USER` back
and forth frequently.

### WORKDIR

[Dockerfile reference for the WORKDIR instruction](../reference/builder.md#workdir)

For clarity and reliability, you should always use absolute paths for your
`WORKDIR`. Also, you should use `WORKDIR` instead of  proliferating
instructions like `RUN cd … && do-something`, which are hard to read,
troubleshoot, and maintain.

### ONBUILD

[Dockerfile reference for the ONBUILD instruction](../reference/builder.md#onbuild)

An `ONBUILD` command executes after the current `Dockerfile` build completes.
`ONBUILD` executes in any child image derived `FROM` the current image.  Think
of the `ONBUILD` command as an instruction the parent `Dockerfile` gives
to the child `Dockerfile`.

A Docker build executes `ONBUILD` commands before any command in a child
`Dockerfile`.

`ONBUILD` is useful for images that are going to be built `FROM` a given
image. For example, you would use `ONBUILD` for a language stack image that
builds arbitrary user software written in that language within the
`Dockerfile`, as you can see in [Ruby’s `ONBUILD` variants](https://github.com/docker-library/ruby/blob/master/2.1/onbuild/Dockerfile).

Images built from `ONBUILD` should get a separate tag, for example:
`ruby:1.9-onbuild` or `ruby:2.0-onbuild`.

Be careful when putting `ADD` or `COPY` in `ONBUILD`. The “onbuild” image will
fail catastrophically if the new build's context is missing the resource being
added. Adding a separate tag, as recommended above, will help mitigate this by
allowing the `Dockerfile` author to make a choice.

## Examples for Official Repositories

These Official Repositories have exemplary `Dockerfile`s:

* [Go](https://registry.hub.docker.com/_/golang/)
* [Perl](https://registry.hub.docker.com/_/perl/)
* [Hy](https://registry.hub.docker.com/_/hylang/)
* [Rails](https://registry.hub.docker.com/_/rails)

## Additional resources:

* [Dockerfile Reference](../reference/builder.md)
* [More about Base Images](baseimages.md)
* [More about Automated Builds](https://docs.docker.com/docker-hub/builds/)
* [Guidelines for Creating Official
Repositories](https://docs.docker.com/docker-hub/official_repos/)

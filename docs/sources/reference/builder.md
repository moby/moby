page_title: Dockerfile Reference
page_description: Dockerfiles use a simple DSL which allows you to automate the steps you would normally manually take to create an image.
page_keywords: builder, docker, Dockerfile, automation, image creation

# Dockerfile Reference

**Docker can build images automatically** by reading the instructions
from a `Dockerfile`. A `Dockerfile` is a text document that contains all
the commands you would normally execute manually in order to build a
Docker image. By calling `docker build` from your terminal, you can have
Docker build your image step by step, executing the instructions
successively.

## Usage

To [*build*](../commandline/cli/#cli-build) an image from a source repository,
create a description file called `Dockerfile` at the root of your repository.
This file will describe the steps to assemble the image.

Then call `docker build` with the path of your source repository as the argument
(for example, `.`):

    $ sudo docker build .

The path to the source repository defines where to find the *context* of
the build. The build is run by the Docker daemon, not by the CLI, so the
whole context must be transferred to the daemon. The Docker CLI reports
"Sending build context to Docker daemon" when the context is sent to the daemon.

> **Warning**
> Avoid using your root directory, `/`, as the root of the source repository. The 
> `docker build` command will use whatever directory contains the Dockerfile as the build
> context (including all of its subdirectories). The build context will be sent to the
> Docker daemon before building the image, which means if you use `/` as the source
> repository, the entire contents of your hard drive will get sent to the daemon (and
> thus to the machine running the daemon). You probably don't want that.

In most cases, it's best to put each Dockerfile in an empty directory, and then add only
the files needed for building that Dockerfile to that directory. To further speed up the
build, you can exclude files and directories by adding a `.dockerignore` file to the same
directory.

You can specify a repository and tag at which to save the new image if
the build succeeds:

    $ sudo docker build -t shykes/myapp .

The Docker daemon will run your steps one-by-one, committing the result
to a new image if necessary, before finally outputting the ID of your
new image. The Docker daemon will automatically clean up the context you
sent.

Note that each instruction is run independently, and causes a new image
to be created - so `RUN cd /tmp` will not have any effect on the next
instructions.

Whenever possible, Docker will re-use the intermediate images,
accelerating `docker build` significantly (indicated by `Using cache`):

    $ docker build -t SvenDowideit/ambassador .
    Uploading context 10.24 kB
    Uploading context
    Step 1 : FROM docker-ut
     ---> cbba202fe96b
    Step 2 : MAINTAINER SvenDowideit@home.org.au
     ---> Using cache
     ---> 51182097be13
    Step 3 : CMD env | grep _TCP= | sed 's/.*_PORT_\([0-9]*\)_TCP=tcp:\/\/\(.*\):\(.*\)/socat TCP4-LISTEN:\1,fork,reuseaddr TCP4:\2:\3 \&/'  | sh && top
     ---> Using cache
     ---> 1a5ffc17324d
    Successfully built 1a5ffc17324d

When you're done with your build, you're ready to look into [*Pushing a
repository to its registry*]( /userguide/dockerrepos/#image-push).

## Format

Here is the format of the `Dockerfile`:

    # Comment
    INSTRUCTION arguments

The Instruction is not case-sensitive, however convention is for them to
be UPPERCASE in order to distinguish them from arguments more easily.

Docker runs the instructions in a `Dockerfile` in order. **The
first instruction must be \`FROM\`** in order to specify the [*Base
Image*](/terms/image/#base-image-def) from which you are building.

Docker will treat lines that *begin* with `#` as a
comment. A `#` marker anywhere else in the line will
be treated as an argument. This allows statements like:

    # Comment
    RUN echo 'we are running some # of cool things'

Here is the set of instructions you can use in a `Dockerfile` for building
images.

## The `.dockerignore` file

If a file named `.dockerignore` exists in the source repository, then it
is interpreted as a newline-separated list of exclusion patterns.
Exclusion patterns match files or directories relative to the source repository
that will be excluded from the context. Globbing is done using Go's
[filepath.Match](http://golang.org/pkg/path/filepath#Match) rules.

The following example shows the use of the `.dockerignore` file to exclude the
`.git` directory from the context. Its effect can be seen in the changed size of
the uploaded context.

    $ docker build .
    Uploading context 18.829 MB
    Uploading context
    Step 0 : FROM busybox
     ---> 769b9341d937
    Step 1 : CMD echo Hello World
     ---> Using cache
     ---> 99cc1ad10469
    Successfully built 99cc1ad10469
    $ echo ".git" > .dockerignore
    $ docker build .
    Uploading context  6.76 MB
    Uploading context
    Step 0 : FROM busybox
     ---> 769b9341d937
    Step 1 : CMD echo Hello World
     ---> Using cache
     ---> 99cc1ad10469
    Successfully built 99cc1ad10469

## FROM

    FROM <image>

Or

    FROM <image>:<tag>

The `FROM` instruction sets the [*Base Image*](/terms/image/#base-image-def)
for subsequent instructions. As such, a valid `Dockerfile` must have `FROM` as
its first instruction. The image can be any valid image – it is especially easy
to start by **pulling an image** from the [*Public Repositories*](
/userguide/dockerrepos/#using-public-repositories).

`FROM` must be the first non-comment instruction in the `Dockerfile`.

`FROM` can appear multiple times within a single `Dockerfile` in order to create
multiple images. Simply make a note of the last image ID output by the commit
before each new `FROM` command.

If no `tag` is given to the `FROM` instruction, `latest` is assumed. If the
used tag does not exist, an error will be returned.

## MAINTAINER

    MAINTAINER <name>

The `MAINTAINER` instruction allows you to set the *Author* field of the
generated images.

## RUN

RUN has 2 forms:

- `RUN <command>` (the command is run in a shell - `/bin/sh -c`)
- `RUN ["executable", "param1", "param2"]` (*exec* form)

The `RUN` instruction will execute any commands in a new layer on top of the
current image and commit the results. The resulting committed image will be
used for the next step in the `Dockerfile`.

Layering `RUN` instructions and generating commits conforms to the core
concepts of Docker where commits are cheap and containers can be created from
any point in an image's history, much like source control.

The *exec* form makes it possible to avoid shell string munging, and to `RUN`
commands using a base image that does not contain `/bin/sh`.

> **Note**:
> To use a different shell, other than '/bin/sh', use the *exec* form
> passing in the desired shell. For example,
> `RUN ["/bin/bash", "-c", "echo hello"]`

The cache for `RUN` instructions isn't invalidated automatically during
the next build. The cache for an instruction like `RUN apt-get
dist-upgrade -y` will be reused during the next build.  The cache for
`RUN` instructions can be invalidated by using the `--no-cache` flag,
for example `docker build --no-cache`.

The cache for `RUN` instructions can be invalidated by `ADD` instructions. See
[below](#add) for details.

### Known Issues (RUN)

- [Issue 783](https://github.com/docker/docker/issues/783) is about file
  permissions problems that can occur when using the AUFS file system. You
  might notice it during an attempt to `rm` a file, for example. The issue
  describes a workaround.

## CMD

The `CMD` instruction has three forms:

- `CMD ["executable","param1","param2"]` (like an *exec*, this is the preferred form)
- `CMD ["param1","param2"]` (as *default parameters to ENTRYPOINT*)
- `CMD command param1 param2` (as a *shell*)

There can only be one `CMD` instruction in a `Dockerfile`. If you list more than one `CMD`
then only the last `CMD` will take effect.

**The main purpose of a `CMD` is to provide defaults for an executing
container.** These defaults can include an executable, or they can omit
the executable, in which case you must specify an `ENTRYPOINT`
instruction as well.

> **Note**:
> If `CMD` is used to provide default arguments for the `ENTRYPOINT` 
> instruction, both the `CMD` and `ENTRYPOINT` instructions should be specified 
> with the JSON array format.

When used in the shell or exec formats, the `CMD` instruction sets the command
to be executed when running the image.

If you use the *shell* form of the `CMD`, then the `<command>` will execute in
`/bin/sh -c`:

    FROM ubuntu
    CMD echo "This is a test." | wc -

If you want to **run your** `<command>` **without a shell** then you must
express the command as a JSON array and give the full path to the executable.
**This array form is the preferred format of `CMD`.** Any additional parameters
must be individually expressed as strings in the array:

    FROM ubuntu
    CMD ["/usr/bin/wc","--help"]

If you would like your container to run the same executable every time, then
you should consider using `ENTRYPOINT` in combination with `CMD`. See
[*ENTRYPOINT*](#entrypoint).

If the user specifies arguments to `docker run` then they will override the
default specified in `CMD`.

> **Note**:
> don't confuse `RUN` with `CMD`. `RUN` actually runs a command and commits
> the result; `CMD` does not execute anything at build time, but specifies
> the intended command for the image.

## EXPOSE

    EXPOSE <port> [<port>...]

The `EXPOSE` instructions informs Docker that the container will listen on the
specified network ports at runtime. Docker uses this information to interconnect
containers using links (see the [Docker User
Guide](/userguide/dockerlinks)).

## ENV

    ENV <key> <value>

The `ENV` instruction sets the environment variable `<key>` to the value
`<value>`. This value will be passed to all future `RUN` instructions. This is
functionally equivalent to prefixing the command with `<key>=<value>`

The environment variables set using `ENV` will persist when a container is run
from the resulting image. You can view the values using `docker inspect`, and
change them using `docker run --env <key>=<value>`.

> **Note**:
> One example where this can cause unexpected consequences, is setting
> `ENV DEBIAN_FRONTEND noninteractive`. Which will persist when the container
> is run interactively; for example: `docker run -t -i image bash`

## ADD

    ADD <src> <dest>

The `ADD` instruction will copy new files from `<src>` and add them to the
container's filesystem at path `<dest>`.

`<src>` must be the path to a file or directory relative to the source directory
being built (also called the *context* of the build) or a remote file URL.

`<dest>` is the absolute path to which the source will be copied inside the
destination container.

All new files and directories are created with a UID and GID of 0.

In the case where `<src>` is a remote file URL, the destination will
have permissions of 600.

> **Note**:
> If you build by passing a `Dockerfile` through STDIN (`docker
> build - < somefile`), there is no build context, so the `Dockerfile`
> can only contain a URL based `ADD` instruction. You can also pass a
> compressed archive through STDIN: (`docker build - < archive.tar.gz`),
> the `Dockerfile` at the root of the archive and the rest of the
> archive will get used at the context of the build.

> **Note**:
> If your URL files are protected using authentication, you
> will need to use `RUN wget`, `RUN curl` or use another tool from
> within the container as the `ADD` instruction does not support
> authentication.

> **Note**:
> The first encountered `ADD` instruction will invalidate the cache for all
> following instructions from the Dockerfile if the contents of `<src>` have
> changed. This includes invalidating the cache for `RUN` instructions.

The copy obeys the following rules:

- The `<src>` path must be inside the *context* of the build;
  you cannot `ADD ../something /something`, because the first step of a
  `docker build` is to send the context directory (and subdirectories) to the
  docker daemon.

- If `<src>` is a URL and `<dest>` does not end with a trailing slash, then a
  file is downloaded from the URL and copied to `<dest>`.

- If `<src>` is a URL and `<dest>` does end with a trailing slash, then the
  filename is inferred from the URL and the file is downloaded to
  `<dest>/<filename>`. For instance, `ADD http://example.com/foobar /` would
  create the file `/foobar`. The URL must have a nontrivial path so that an
  appropriate filename can be discovered in this case (`http://example.com`
  will not work).

- If `<src>` is a directory, the entire directory is copied, including
  filesystem metadata.

- If `<src>` is a *local* tar archive in a recognized compression format
  (identity, gzip, bzip2 or xz) then it is unpacked as a directory. Resources
  from *remote* URLs are **not** decompressed. When a directory is copied or
  unpacked, it has the same behavior as `tar -x`: the result is the union of:

    1. Whatever existed at the destination path and
    2. The contents of the source tree, with conflicts resolved in favor
       of "2." on a file-by-file basis.

- If `<src>` is any other kind of file, it is copied individually along with
  its metadata. In this case, if `<dest>` ends with a trailing slash `/`, it
  will be considered a directory and the contents of `<src>` will be written
  at `<dest>/base(<src>)`.

- If `<dest>` does not end with a trailing slash, it will be considered a
  regular file and the contents of `<src>` will be written at `<dest>`.

- If `<dest>` doesn't exist, it is created along with all missing directories
  in its path.

## COPY

    COPY <src> <dest>

The `COPY` instruction will copy new files from `<src>` and add them to the
container's filesystem at path `<dest>`.

`<src>` must be the path to a file or directory relative to the source directory
being built (also called the *context* of the build).

`<dest>` is the absolute path to which the source will be copied inside the
destination container.

All new files and directories are created with a UID and GID of 0.

> **Note**:
> If you build using STDIN (`docker build - < somefile`), there is no
> build context, so `COPY` can't be used.

The copy obeys the following rules:

- The `<src>` path must be inside the *context* of the build;
  you cannot `COPY ../something /something`, because the first step of a
  `docker build` is to send the context directory (and subdirectories) to the
  docker daemon.

- If `<src>` is a directory, the entire directory is copied, including
  filesystem metadata.

- If `<src>` is any other kind of file, it is copied individually along with
  its metadata. In this case, if `<dest>` ends with a trailing slash `/`, it
  will be considered a directory and the contents of `<src>` will be written
  at `<dest>/base(<src>)`.

- If `<dest>` does not end with a trailing slash, it will be considered a
  regular file and the contents of `<src>` will be written at `<dest>`.

- If `<dest>` doesn't exist, it is created along with all missing directories
  in its path.

## ENTRYPOINT

ENTRYPOINT has two forms:

- `ENTRYPOINT ["executable", "param1", "param2"]`
  (like an *exec*, the preferred form)
- `ENTRYPOINT command param1 param2`
  (as a *shell*)

There can only be one `ENTRYPOINT` in a `Dockerfile`. If you have more
than one `ENTRYPOINT`, then only the last one in the `Dockerfile` will
have an effect.

An `ENTRYPOINT` helps you to configure a container that you can run as
an executable. That is, when you specify an `ENTRYPOINT`, then the whole
container runs as if it was just that executable.

Unlike the behavior of the `CMD` instruction, The `ENTRYPOINT`
instruction adds an entry command that will **not** be overwritten when
arguments are passed to `docker run`. This allows arguments to be passed
to the entry point, i.e.  `docker run <image> -d` will pass the `-d`
argument to the entry point.

You can specify parameters either in the `ENTRYPOINT` JSON array (as in
"like an exec" above), or by using a `CMD` instruction. Parameters in
the `ENTRYPOINT` instruction will not be overridden by the `docker run`
arguments, but parameters specified via a `CMD` instruction will be
overridden by `docker run` arguments.

Like a `CMD`, you can specify a plain string for the `ENTRYPOINT` and it
will execute in `/bin/sh -c`:

    FROM ubuntu
    ENTRYPOINT ls -l

For example, that `Dockerfile`'s image will *always* take a directory as
an input and return a directory listing. If you wanted to make this
optional but default, you could use a `CMD` instruction:

    FROM ubuntu
    CMD ["-l"]
    ENTRYPOINT ["ls"]

> **Note**:
> It is preferable to use the JSON array format for specifying
> `ENTRYPOINT` instructions.

## VOLUME

    VOLUME ["/data"]

The `VOLUME` instruction will create a mount point with the specified name
and mark it as holding externally mounted volumes from native host or other
containers. The value can be a JSON array, `VOLUME ["/var/log/"]`, or a plain
string, `VOLUME /var/log`. For more information/examples and mounting
instructions via the Docker client, refer to [*Share Directories via Volumes*](
/userguide/dockervolumes/#volume-def) documentation.

## USER

    USER daemon

The `USER` instruction sets the user name or UID to use when running the image
and for any following `RUN` directives.

## WORKDIR

    WORKDIR /path/to/workdir

The `WORKDIR` instruction sets the working directory for any `RUN`, `CMD` and
`ENTRYPOINT` instructions that follow it in the `Dockerfile`.

It can be used multiple times in the one `Dockerfile`. If a relative path
is provided, it will be relative to the path of the previous `WORKDIR`
instruction. For example:

    WORKDIR /a
    WORKDIR b
    WORKDIR c
    RUN pwd

The output of the final `pwd` command in this Dockerfile would be
`/a/b/c`.

## ONBUILD

    ONBUILD [INSTRUCTION]

The `ONBUILD` instruction adds to the image a *trigger* instruction to
be executed at a later time, when the image is used as the base for
another build. The trigger will be executed in the context of the
downstream build, as if it had been inserted immediately after the
`FROM` instruction in the downstream `Dockerfile`.

Any build instruction can be registered as a trigger.

This is useful if you are building an image which will be used as a base
to build other images, for example an application build environment or a
daemon which may be customized with user-specific configuration.

For example, if your image is a reusable Python application builder, it
will require application source code to be added in a particular
directory, and it might require a build script to be called *after*
that. You can't just call `ADD` and `RUN` now, because you don't yet
have access to the application source code, and it will be different for
each application build. You could simply provide application developers
with a boilerplate `Dockerfile` to copy-paste into their application, but
that is inefficient, error-prone and difficult to update because it
mixes with application-specific code.

The solution is to use `ONBUILD` to register advance instructions to
run later, during the next build stage.

Here's how it works:

1. When it encounters an `ONBUILD` instruction, the builder adds a
   trigger to the metadata of the image being built. The instruction
   does not otherwise affect the current build.
2. At the end of the build, a list of all triggers is stored in the
   image manifest, under the key `OnBuild`. They can be inspected with
   the `docker inspect` command.
3. Later the image may be used as a base for a new build, using the
   `FROM` instruction. As part of processing the `FROM` instruction,
   the downstream builder looks for `ONBUILD` triggers, and executes
   them in the same order they were registered. If any of the triggers
   fail, the `FROM` instruction is aborted which in turn causes the
   build to fail. If all triggers succeed, the `FROM` instruction
   completes and the build continues as usual.
4. Triggers are cleared from the final image after being executed. In
   other words they are not inherited by "grand-children" builds.

For example you might add something like this:

    [...]
    ONBUILD ADD . /app/src
    ONBUILD RUN /usr/local/bin/python-build --dir /app/src
    [...]

> **Warning**: Chaining `ONBUILD` instructions using `ONBUILD ONBUILD` isn't allowed.

> **Warning**: The `ONBUILD` instruction may not trigger `FROM` or `MAINTAINER` instructions.

## Dockerfile Examples

    # Nginx
    #
    # VERSION               0.0.1

    FROM      ubuntu
    MAINTAINER Victor Vieux <victor@docker.com>

    RUN apt-get update && apt-get install -y inotify-tools nginx apache2 openssh-server

    # Firefox over VNC
    #
    # VERSION               0.3

    FROM ubuntu

    # Install vnc, xvfb in order to create a 'fake' display and firefox
    RUN apt-get update && apt-get install -y x11vnc xvfb firefox
    RUN mkdir /.vnc
    # Setup a password
    RUN x11vnc -storepasswd 1234 ~/.vnc/passwd
    # Autostart firefox (might not be the best way, but it does the trick)
    RUN bash -c 'echo "firefox" >> /.bashrc'

    EXPOSE 5900
    CMD    ["x11vnc", "-forever", "-usepw", "-create"]

    # Multiple images example
    #
    # VERSION               0.1

    FROM ubuntu
    RUN echo foo > bar
    # Will output something like ===> 907ad6c2736f

    FROM ubuntu
    RUN echo moo > oink
    # Will output something like ===> 695d7793cbe4

    # You᾿ll now have two images, 907ad6c2736f with /bar, and 695d7793cbe4 with
    # /oink.


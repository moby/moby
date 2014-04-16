page_title: Dockerfile Reference
page_description: Dockerfiles use a simple DSL which allows you to automate the steps you would normally manually take to create an image.
page_keywords: builder, docker, Dockerfile, automation, image creation

# Dockerfile Reference

**Docker can act as a builder** and read instructions from a text
`Dockerfile` to automate the steps you would
otherwise take manually to create an image. Executing
`docker build` will run your steps and commit them
along the way, giving you a final image.

## Usage

To [*build*](../commandline/cli/#cli-build) an image from a source
repository, create a description file called `Dockerfile`
at the root of your repository. This file will describe the
steps to assemble the image.

Then call `docker build` with the path of your
source repository as argument (for example, `.`):

> `sudo docker build .`

The path to the source repository defines where to find the *context* of
the build. The build is run by the Docker daemon, not by the CLI, so the
whole context must be transferred to the daemon. The Docker CLI reports
"Uploading context" when the context is sent to the daemon.

You can specify a repository and tag at which to save the new image if
the build succeeds:

> `sudo docker build -t shykes/myapp .`

The Docker daemon will run your steps one-by-one, committing the result
to a new image if necessary, before finally outputting the ID of your
new image. The Docker daemon will automatically clean up the context you
sent.

Note that each instruction is run independently, and causes a new image
to be created - so `RUN cd /tmp` will not have any
effect on the next instructions.

Whenever possible, Docker will re-use the intermediate images,
accelerating `docker build` significantly (indicated
by `Using cache`):

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

When you’re done with your build, you’re ready to look into [*Pushing a
repository to its
registry*](../../use/workingwithrepository/#image-push).

## Format

Here is the format of the Dockerfile:

    # Comment
    INSTRUCTION arguments

The Instruction is not case-sensitive, however convention is for them to
be UPPERCASE in order to distinguish them from arguments more easily.

Docker evaluates the instructions in a Dockerfile in order. **The first
instruction must be \`FROM\`** in order to specify the [*Base
Image*](../../terms/image/#base-image-def) from which you are building.

Docker will treat lines that *begin* with `#` as a
comment. A `#` marker anywhere else in the line will
be treated as an argument. This allows statements like:

    # Comment
    RUN echo 'we are running some # of cool things'

Here is the set of instructions you can use in a `Dockerfile`
for building images.

## `FROM`

> `FROM <image>`

Or

> `FROM <image>:<tag>`

The `FROM` instruction sets the [*Base
Image*](../../terms/image/#base-image-def) for subsequent instructions.
As such, a valid Dockerfile must have `FROM` as its
first instruction. The image can be any valid image – it is especially
easy to start by **pulling an image** from the [*Public
Repositories*](../../use/workingwithrepository/#using-public-repositories).

`FROM` must be the first non-comment instruction in
the `Dockerfile`.

`FROM` can appear multiple times within a single
Dockerfile in order to create multiple images. Simply make a note of the
last image id output by the commit before each new `FROM`
command.

If no `tag` is given to the `FROM`
instruction, `latest` is assumed. If the
used tag does not exist, an error will be returned.

## `MAINTAINER`

> `MAINTAINER <name>`

The `MAINTAINER` instruction allows you to set the
*Author* field of the generated images.

## `RUN`

RUN has 2 forms:

-   `RUN <command>` (the command is run in a shell -
    `/bin/sh -c`)
-   `RUN ["executable", "param1", "param2"]` (*exec*
    form)

The `RUN` instruction will execute any commands in a
new layer on top of the current image and commit the results. The
resulting committed image will be used for the next step in the
Dockerfile.

Layering `RUN` instructions and generating commits
conforms to the core concepts of Docker where commits are cheap and
containers can be created from any point in an image’s history, much
like source control.

The *exec* form makes it possible to avoid shell string munging, and to
`RUN` commands using a base image that does not
contain `/bin/sh`.

### Known Issues (RUN)

-   [Issue 783](https://github.com/dotcloud/docker/issues/783) is about
    file permissions problems that can occur when using the AUFS file
    system. You might notice it during an attempt to `rm`
 a file, for example. The issue describes a workaround.
-   [Issue 2424](https://github.com/dotcloud/docker/issues/2424) Locale
    will not be set automatically.

## `CMD`

CMD has three forms:

-   `CMD ["executable","param1","param2"]` (like an
    *exec*, preferred form)
-   `CMD ["param1","param2"]` (as *default
    parameters to ENTRYPOINT*)
-   `CMD command param1 param2` (as a *shell*)

There can only be one CMD in a Dockerfile. If you list more than one CMD
then only the last CMD will take effect.

**The main purpose of a CMD is to provide defaults for an executing
container.** These defaults can include an executable, or they can omit
the executable, in which case you must specify an ENTRYPOINT as well.

When used in the shell or exec formats, the `CMD`
instruction sets the command to be executed when running the image.

If you use the *shell* form of the CMD, then the `<command>`
will execute in `/bin/sh -c`:

    FROM ubuntu
    CMD echo "This is a test." | wc -

If you want to **run your** `<command>` **without a
shell** then you must express the command as a JSON array and give the
full path to the executable. **This array form is the preferred format
of CMD.** Any additional parameters must be individually expressed as
strings in the array:

    FROM ubuntu
    CMD ["/usr/bin/wc","--help"]

If you would like your container to run the same executable every time,
then you should consider using `ENTRYPOINT` in
combination with `CMD`. See
[*ENTRYPOINT*](#dockerfile-entrypoint).

If the user specifies arguments to `docker run` then
they will override the default specified in CMD.

Note

Don’t confuse `RUN` with `CMD`.
`RUN` actually runs a command and commits the
result; `CMD` does not execute anything at build
time, but specifies the intended command for the image.

## `EXPOSE`

> `EXPOSE <port> [<port>...]`

The `EXPOSE` instructions informs Docker that the
container will listen on the specified network ports at runtime. Docker
uses this information to interconnect containers using links (see
[*links*](../../use/working_with_links_names/#working-with-links-names)),
and to setup port redirection on the host system (see [*Redirect
Ports*](../../use/port_redirection/#port-redirection)).

## `ENV`

> `ENV <key> <value>`

The `ENV` instruction sets the environment variable
`<key>` to the value `<value>`.
This value will be passed to all future `RUN`
instructions. This is functionally equivalent to prefixing the command
with `<key>=<value>`

The environment variables set using `ENV` will
persist when a container is run from the resulting image. You can view
the values using `docker inspect`, and change them
using `docker run --env <key>=<value>`.

Note

One example where this can cause unexpected consequenses, is setting
`ENV DEBIAN_FRONTEND noninteractive`. Which will
persist when the container is run interactively; for example:
`docker run -t -i image bash`

## `ADD`

> `ADD <src> <dest>`

The `ADD` instruction will copy new files from
\<src\> and add them to the container’s filesystem at path
`<dest>`.

`<src>` must be the path to a file or directory
relative to the source directory being built (also called the *context*
of the build) or a remote file URL.

`<dest>` is the absolute path to which the source
will be copied inside the destination container.

All new files and directories are created with mode 0755, uid and gid 0.

Note

if you build using STDIN (`docker build - < somefile`
.literal}), there is no build context, so the Dockerfile can only
contain an URL based ADD statement.

Note

if your URL files are protected using authentication, you will need to
use an `RUN wget` , `RUN curl`
or other tool from within the container as ADD does not support
authentication.

The copy obeys the following rules:

-   The `<src>` path must be inside the *context* of
    the build; you cannot `ADD ../something /something`
, because the first step of a `docker build`
 is to send the context directory (and subdirectories) to
    the docker daemon.

-   If `<src>` is a URL and `<dest>`
 does not end with a trailing slash, then a file is
    downloaded from the URL and copied to `<dest>`.

-   If `<src>` is a URL and `<dest>`
 does end with a trailing slash, then the filename is
    inferred from the URL and the file is downloaded to
    `<dest>/<filename>`. For instance,
    `ADD http://example.com/foobar /` would create
    the file `/foobar`. The URL must have a
    nontrivial path so that an appropriate filename can be discovered in
    this case (`http://example.com` will not work).

-   If `<src>` is a directory, the entire directory
    is copied, including filesystem metadata.

-   If `<src>` is a *local* tar archive in a
    recognized compression format (identity, gzip, bzip2 or xz) then it
    is unpacked as a directory. Resources from *remote* URLs are **not**
    decompressed.

    When a directory is copied or unpacked, it has the same behavior as
    `tar -x`: the result is the union of

    1.  whatever existed at the destination path and
    2.  the contents of the source tree,

    with conflicts resolved in favor of "2." on a file-by-file basis.

-   If `<src>` is any other kind of file, it is
    copied individually along with its metadata. In this case, if
    `<dest>` ends with a trailing slash
    `/`, it will be considered a directory and the
    contents of `<src>` will be written at
    `<dest>/base(<src>)`.

-   If `<dest>` does not end with a trailing slash,
    it will be considered a regular file and the contents of
    `<src>` will be written at `<dest>`
.

-   If `<dest>` doesn’t exist, it is created along
    with all missing directories in its path.

## `ENTRYPOINT`

ENTRYPOINT has two forms:

-   `ENTRYPOINT ["executable", "param1", "param2"]`
    (like an *exec*, preferred form)
-   `ENTRYPOINT command param1 param2` (as a
    *shell*)

There can only be one `ENTRYPOINT` in a Dockerfile.
If you have more than one `ENTRYPOINT`, then only
the last one in the Dockerfile will have an effect.

An `ENTRYPOINT` helps you to configure a container
that you can run as an executable. That is, when you specify an
`ENTRYPOINT`, then the whole container runs as if it
was just that executable.

The `ENTRYPOINT` instruction adds an entry command
that will **not** be overwritten when arguments are passed to
`docker run`, unlike the behavior of `CMD`
.literal}. This allows arguments to be passed to the entrypoint. i.e.
`docker run <image> -d` will pass the "-d" argument
to the ENTRYPOINT.

You can specify parameters either in the ENTRYPOINT JSON array (as in
"like an exec" above), or by using a CMD statement. Parameters in the
ENTRYPOINT will not be overridden by the `docker run`
arguments, but parameters specified via CMD will be overridden
by `docker run` arguments.

Like a `CMD`, you can specify a plain string for the
ENTRYPOINT and it will execute in `/bin/sh -c`:

    FROM ubuntu
    ENTRYPOINT wc -l -

For example, that Dockerfile’s image will *always* take stdin as input
("-") and print the number of lines ("-l"). If you wanted to make this
optional but default, you could use a CMD:

    FROM ubuntu
    CMD ["-l", "-"]
    ENTRYPOINT ["/usr/bin/wc"]

## `VOLUME`

> `VOLUME ["/data"]`

The `VOLUME` instruction will create a mount point
with the specified name and mark it as holding externally mounted
volumes from native host or other containers. For more
information/examples and mounting instructions via docker client, refer
to [*Share Directories via
Volumes*](../../use/working_with_volumes/#volume-def) documentation.

## `USER`

> `USER daemon`

The `USER` instruction sets the username or UID to
use when running the image.

## `WORKDIR`

> `WORKDIR /path/to/workdir`

The `WORKDIR` instruction sets the working directory
for the `RUN`, `CMD` and
`ENTRYPOINT` Dockerfile commands that follow it.

It can be used multiple times in the one Dockerfile. If a relative path
is provided, it will be relative to the path of the previous
`WORKDIR` instruction. For example:

> WORKDIR /a WORKDIR b WORKDIR c RUN pwd

The output of the final `pwd` command in this
Dockerfile would be `/a/b/c`.

## `ONBUILD`

> `ONBUILD [INSTRUCTION]`

The `ONBUILD` instruction adds to the image a
"trigger" instruction to be executed at a later time, when the image is
used as the base for another build. The trigger will be executed in the
context of the downstream build, as if it had been inserted immediately
after the *FROM* instruction in the downstream Dockerfile.

Any build instruction can be registered as a trigger.

This is useful if you are building an image which will be used as a base
to build other images, for example an application build environment or a
daemon which may be customized with user-specific configuration.

For example, if your image is a reusable python application builder, it
will require application source code to be added in a particular
directory, and it might require a build script to be called *after*
that. You can’t just call *ADD* and *RUN* now, because you don’t yet
have access to the application source code, and it will be different for
each application build. You could simply provide application developers
with a boilerplate Dockerfile to copy-paste into their application, but
that is inefficient, error-prone and difficult to update because it
mixes with application-specific code.

The solution is to use *ONBUILD* to register in advance instructions to
run later, during the next build stage.

Here’s how it works:

1.  When it encounters an *ONBUILD* instruction, the builder adds a
    trigger to the metadata of the image being built. The instruction
    does not otherwise affect the current build.
2.  At the end of the build, a list of all triggers is stored in the
    image manifest, under the key *OnBuild*. They can be inspected with
    *docker inspect*.
3.  Later the image may be used as a base for a new build, using the
    *FROM* instruction. As part of processing the *FROM* instruction,
    the downstream builder looks for *ONBUILD* triggers, and executes
    them in the same order they were registered. If any of the triggers
    fail, the *FROM* instruction is aborted which in turn causes the
    build to fail. If all triggers succeed, the FROM instruction
    completes and the build continues as usual.
4.  Triggers are cleared from the final image after being executed. In
    other words they are not inherited by "grand-children" builds.

For example you might add something like this:

    [...]
    ONBUILD ADD . /app/src
    ONBUILD RUN /usr/local/bin/python-build --dir /app/src
    [...]

Warning

Chaining ONBUILD instructions using ONBUILD ONBUILD isn’t allowed.

Warning

ONBUILD may not trigger FROM or MAINTAINER instructions.

## Dockerfile Examples

    # Nginx
    #
    # VERSION               0.0.1

    FROM      ubuntu
    MAINTAINER Guillaume J. Charmes <guillaume@docker.com>

    # make sure the package repository is up to date
    RUN echo "deb http://archive.ubuntu.com/ubuntu precise main universe" > /etc/apt/sources.list
    RUN apt-get update

    RUN apt-get install -y inotify-tools nginx apache2 openssh-server

    # Firefox over VNC
    #
    # VERSION               0.3

    FROM ubuntu
    # make sure the package repository is up to date
    RUN echo "deb http://archive.ubuntu.com/ubuntu precise main universe" > /etc/apt/sources.list
    RUN apt-get update

    # Install vnc, xvfb in order to create a 'fake' display and firefox
    RUN apt-get install -y x11vnc xvfb firefox
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

    # You'll now have two images, 907ad6c2736f with /bar, and 695d7793cbe4 with
    # /oink.

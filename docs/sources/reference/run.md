page_title: Docker Run Reference 
page_description: Configure containers at runtime
page_keywords: docker, run, configure, runtime

# [Docker Run Reference](#id2)

**Docker runs processes in isolated containers**. When an operator
executes `docker run`, she starts a process with its
own file system, its own networking, and its own isolated process tree.
The [*Image*](../../terms/image/#image-def) which starts the process may
define defaults related to the binary to run, the networking to expose,
and more, but `docker run` gives final control to
the operator who starts the container from the image. That’s the main
reason [*run*](../commandline/cli/#cli-run) has more options than any
other `docker` command.

Every one of the [*Examples*](../../examples/#example-list) shows
running containers, and so here we try to give more in-depth guidance.

## [General Form](#id3)

As you’ve seen in the [*Examples*](../../examples/#example-list), the
basic run command takes this form:

    docker run [OPTIONS] IMAGE[:TAG] [COMMAND] [ARG...]

To learn how to interpret the types of `[OPTIONS]`,
see [*Option types*](../commandline/cli/#cli-options).

The list of `[OPTIONS]` breaks down into two groups:

1.  Settings exclusive to operators, including:
    -   Detached or Foreground running,
    -   Container Identification,
    -   Network settings, and
    -   Runtime Constraints on CPU and Memory
    -   Privileges and LXC Configuration

2.  Setting shared between operators and developers, where operators can
    override defaults developers set in images at build time.

Together, the `docker run [OPTIONS]` give complete
control over runtime behavior to the operator, allowing them to override
all defaults set by the developer during `docker build`
and nearly all the defaults set by the Docker runtime itself.

## [Operator Exclusive Options](#id4)

Only the operator (the person executing `docker run`) can set the
following options.

-   [Detached vs Foreground](#detached-vs-foreground)
    -   [Detached (-d)](#detached-d)
    -   [Foreground](#foreground)
-   [Container Identification](#container-identification)
    -   [Name (–name)](#name-name)
    -   [PID Equivalent](#pid-equivalent)
-   [Network Settings](#network-settings)
-   [Clean Up (–rm)](#clean-up-rm)
-   [Runtime Constraints on CPU and
    Memory](#runtime-constraints-on-cpu-and-memory)
-   [Runtime Privilege and LXC
    Configuration](#runtime-privilege-and-lxc-configuration)

### [Detached vs Foreground](#id2)

When starting a Docker container, you must first decide if you want to
run the container in the background in a "detached" mode or in the
default foreground mode:

    -d=false: Detached mode: Run container in the background, print new container id

#### [Detached (-d)](#id3)

In detached mode (`-d=true` or just `-d`), all I/O should be done
through network connections or shared volumes because the container is
no longer listening to the commandline where you executed `docker run`.
You can reattach to a detached container with `docker`
[*attach*](../commandline/cli/#cli-attach). If you choose to run a
container in the detached mode, then you cannot use the `--rm` option.

#### [Foreground](#id4)

In foreground mode (the default when `-d` is not
specified), `docker run` can start the process in
the container and attach the console to the process’s standard input,
output, and standard error. It can even pretend to be a TTY (this is
what most commandline executables expect) and pass along signals. All of
that is configurable:

    -a=[]           : Attach to ``stdin``, ``stdout`` and/or ``stderr``
    -t=false        : Allocate a pseudo-tty
    --sig-proxy=true: Proxify all received signal to the process (even in non-tty mode)
    -i=false        : Keep STDIN open even if not attached

If you do not specify `-a` then Docker will [attach
everything
(stdin,stdout,stderr)](https://github.com/dotcloud/docker/blob/75a7f4d90cde0295bcfb7213004abce8d4779b75/commands.go#L1797).
You can specify to which of the three standard streams
(`stdin`, `stdout`,
`stderr`) you’d like to connect instead, as in:

    docker run -a stdin -a stdout -i -t ubuntu /bin/bash

For interactive processes (like a shell) you will typically want a tty
as well as persistent standard input (`stdin`), so
you’ll use `-i -t` together in most interactive
cases.

### [Container Identification](#id5)

#### [Name (–name)](#id6)

The operator can identify a container in three ways:

-   UUID long identifier
    ("f78375b1c487e03c9438c729345e54db9d20cfa2ac1fc3494b6eb60872e74778")
-   UUID short identifier ("f78375b1c487")
-   Name ("evil\_ptolemy")

The UUID identifiers come from the Docker daemon, and if you do not
assign a name to the container with `--name` then
the daemon will also generate a random string name too. The name can
become a handy way to add meaning to a container since you can use this
name when defining
[*links*](../../use/working_with_links_names/#working-with-links-names)
(or any other place you need to identify a container). This works for
both background and foreground Docker containers.

#### [PID Equivalent](#id7)

And finally, to help with automation, you can have Docker write the
container ID out to a file of your choosing. This is similar to how some
programs might write out their process ID to a file (you’ve seen them as
PID files):

    --cidfile="": Write the container ID to the file

### [Network Settings](#id8)

    -n=true   : Enable networking for this container
    --dns=[]  : Set custom dns servers for the container

By default, all containers have networking enabled and they can make any
outgoing connections. The operator can completely disable networking
with `docker run -n` which disables all incoming and
outgoing networking. In cases like this, you would perform I/O through
files or STDIN/STDOUT only.

Your container will use the same DNS servers as the host by default, but
you can override this with `--dns`.

### [Clean Up (–rm)](#id9)

By default a container’s file system persists even after the container
exits. This makes debugging a lot easier (since you can inspect the
final state) and you retain all your data by default. But if you are
running short-term **foreground** processes, these container file
systems can really pile up. If instead you’d like Docker to
**automatically clean up the container and remove the file system when
the container exits**, you can add the `--rm` flag:

    --rm=false: Automatically remove the container when it exits (incompatible with -d)

### [Runtime Constraints on CPU and Memory](#id10)

The operator can also adjust the performance parameters of the
container:

    -m="": Memory limit (format: <number><optional unit>, where unit = b, k, m or g)
    -c=0 : CPU shares (relative weight)

The operator can constrain the memory available to a container easily
with `docker run -m`. If the host supports swap
memory, then the `-m` memory setting can be larger
than physical RAM.

Similarly the operator can increase the priority of this container with
the `-c` option. By default, all containers run at
the same priority and get the same proportion of CPU cycles, but you can
tell the kernel to give more shares of CPU time to one or more
containers when you start them via Docker.

### [Runtime Privilege and LXC Configuration](#id11)

    --privileged=false: Give extended privileges to this container
    --lxc-conf=[]: (lxc exec-driver only) Add custom lxc options --lxc-conf="lxc.cgroup.cpuset.cpus = 0,1"

By default, Docker containers are "unprivileged" and cannot, for
example, run a Docker daemon inside a Docker container. This is because
by default a container is not allowed to access any devices, but a
"privileged" container is given access to all devices (see
[lxc-template.go](https://github.com/dotcloud/docker/blob/master/execdriver/lxc/lxc_template.go)
and documentation on [cgroups
devices](https://www.kernel.org/doc/Documentation/cgroups/devices.txt)).

When the operator executes `docker run --privileged`, Docker will enable
to access to all devices on the host as well as set some configuration
in AppArmor to allow the container nearly all the same access to the
host as processes running outside containers on the host. Additional
information about running with `--privileged` is available on the
[Docker
Blog](http://blog.docker.io/2013/09/docker-can-now-run-within-docker/).

If the Docker daemon was started using the `lxc`
exec-driver (`docker -d --exec-driver=lxc`) then the
operator can also specify LXC options using one or more
`--lxc-conf` parameters. These can be new parameters
or override existing parameters from the
[lxc-template.go](https://github.com/dotcloud/docker/blob/master/execdriver/lxc/lxc_template.go).
Note that in the future, a given host’s Docker daemon may not use LXC,
so this is an implementation-specific configuration meant for operators
already familiar with using LXC directly.

## Overriding `Dockerfile` Image Defaults

When a developer builds an image from a
[*Dockerfile*](../builder/#dockerbuilder) or when she commits it, the
developer can set a number of default parameters that take effect when
the image starts up as a container.

Four of the `Dockerfile` commands cannot be
overridden at runtime: `FROM, MAINTAINER, RUN`, and
`ADD`. Everything else has a corresponding override
in `docker run`. We’ll go through what the developer
might have set in each `Dockerfile` instruction and
how the operator can override that setting.

-   [CMD (Default Command or Options)](#cmd-default-command-or-options)
-   [ENTRYPOINT (Default Command to Execute at
    Runtime](#entrypoint-default-command-to-execute-at-runtime)
-   [EXPOSE (Incoming Ports)](#expose-incoming-ports)
-   [ENV (Environment Variables)](#env-environment-variables)
-   [VOLUME (Shared Filesystems)](#volume-shared-filesystems)
-   [USER](#user)
-   [WORKDIR](#workdir)

### [CMD (Default Command or Options)](#id12)

Recall the optional `COMMAND` in the Docker
commandline:

    docker run [OPTIONS] IMAGE[:TAG] [COMMAND] [ARG...]

This command is optional because the person who created the
`IMAGE` may have already provided a default
`COMMAND` using the `Dockerfile`
`CMD`. As the operator (the person running a
container from the image), you can override that `CMD`
just by specifying a new `COMMAND`.

If the image also specifies an `ENTRYPOINT` then the
`CMD` or `COMMAND` get appended
as arguments to the `ENTRYPOINT`.

### [ENTRYPOINT (Default Command to Execute at Runtime](#id13)

    --entrypoint="": Overwrite the default entrypoint set by the image

The ENTRYPOINT of an image is similar to a `COMMAND` because it
specifies what executable to run when the container starts, but it is
(purposely) more difficult to override. The `ENTRYPOINT` gives a
container its default nature or behavior, so that when you set an
`ENTRYPOINT` you can run the container *as if it were that binary*,
complete with default options, and you can pass in more options via the
`COMMAND`. But, sometimes an operator may want to run something else
inside the container, so you can override the default `ENTRYPOINT` at
runtime by using a string to specify the new `ENTRYPOINT`. Here is an
example of how to run a shell in a container that has been set up to
automatically run something else (like `/usr/bin/redis-server`):

    docker run -i -t --entrypoint /bin/bash example/redis

or two examples of how to pass more parameters to that ENTRYPOINT:

    docker run -i -t --entrypoint /bin/bash example/redis -c ls -l
    docker run -i -t --entrypoint /usr/bin/redis-cli example/redis --help

### [EXPOSE (Incoming Ports)](#id14)

The `Dockerfile` doesn’t give much control over
networking, only providing the `EXPOSE` instruction
to give a hint to the operator about what incoming ports might provide
services. The following options work with or override the
`Dockerfile`‘s exposed defaults:

    --expose=[]: Expose a port from the container
                without publishing it to your host
    -P=false   : Publish all exposed ports to the host interfaces
    -p=[]      : Publish a container᾿s port to the host (format:
                 ip:hostPort:containerPort | ip::containerPort |
                 hostPort:containerPort)
                 (use 'docker port' to see the actual mapping)
    --link=""  : Add link to another container (name:alias)

As mentioned previously, `EXPOSE` (and
`--expose`) make a port available **in** a container
for incoming connections. The port number on the inside of the container
(where the service listens) does not need to be the same number as the
port exposed on the outside of the container (where clients connect), so
inside the container you might have an HTTP service listening on port 80
(and so you `EXPOSE 80` in the
`Dockerfile`), but outside the container the port
might be 42800.

To help a new client container reach the server container’s internal
port operator `--expose`‘d by the operator or
`EXPOSE`‘d by the developer, the operator has three
choices: start the server container with `-P` or
`-p,` or start the client container with
`--link`.

If the operator uses `-P` or `-p`
then Docker will make the exposed port accessible on the host
and the ports will be available to any client that can reach the host.
To find the map between the host ports and the exposed ports, use
`docker port`)

If the operator uses `--link` when starting the new
client container, then the client container can access the exposed port
via a private networking interface. Docker will set some environment
variables in the client container to help indicate which interface and
port to use.

### [ENV (Environment Variables)](#id15)

The operator can **set any environment variable** in the container by
using one or more `-e` flags, even overriding those
already defined by the developer with a Dockefile `ENV`:

    $ docker run -e "deep=purple" --rm ubuntu /bin/bash -c export
    declare -x HOME="/"
    declare -x HOSTNAME="85bc26a0e200"
    declare -x OLDPWD
    declare -x PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
    declare -x PWD="/"
    declare -x SHLVL="1"
    declare -x container="lxc"
    declare -x deep="purple"

Similarly the operator can set the **hostname** with `-h`.

`--link name:alias` also sets environment variables,
using the *alias* string to define environment variables within the
container that give the IP and PORT information for connecting to the
service container. Let’s imagine we have a container running Redis:

    # Start the service container, named redis-name
    $ docker run -d --name redis-name dockerfiles/redis
    4241164edf6f5aca5b0e9e4c9eccd899b0b8080c64c0cd26efe02166c73208f3

    # The redis-name container exposed port 6379
    $ docker ps
    CONTAINER ID        IMAGE                      COMMAND                CREATED             STATUS              PORTS               NAMES
    4241164edf6f        dockerfiles/redis:latest   /redis-stable/src/re   5 seconds ago       Up 4 seconds        6379/tcp            redis-name

    # Note that there are no public ports exposed since we didn᾿t use -p or -P
    $ docker port 4241164edf6f 6379
    2014/01/25 00:55:38 Error: No public port '6379' published for 4241164edf6f

Yet we can get information about the Redis container’s exposed ports
with `--link`. Choose an alias that will form a
valid environment variable!

    $ docker run --rm --link redis-name:redis_alias --entrypoint /bin/bash dockerfiles/redis -c export
    declare -x HOME="/"
    declare -x HOSTNAME="acda7f7b1cdc"
    declare -x OLDPWD
    declare -x PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
    declare -x PWD="/"
    declare -x REDIS_ALIAS_NAME="/distracted_wright/redis"
    declare -x REDIS_ALIAS_PORT="tcp://172.17.0.32:6379"
    declare -x REDIS_ALIAS_PORT_6379_TCP="tcp://172.17.0.32:6379"
    declare -x REDIS_ALIAS_PORT_6379_TCP_ADDR="172.17.0.32"
    declare -x REDIS_ALIAS_PORT_6379_TCP_PORT="6379"
    declare -x REDIS_ALIAS_PORT_6379_TCP_PROTO="tcp"
    declare -x SHLVL="1"
    declare -x container="lxc"

And we can use that information to connect from another container as a
client:

    $ docker run -i -t --rm --link redis-name:redis_alias --entrypoint /bin/bash dockerfiles/redis -c '/redis-stable/src/redis-cli -h $REDIS_ALIAS_PORT_6379_TCP_ADDR -p $REDIS_ALIAS_PORT_6379_TCP_PORT'
    172.17.0.32:6379>

### [VOLUME (Shared Filesystems)](#id16)

    -v=[]: Create a bind mount with: [host-dir]:[container-dir]:[rw|ro].
           If "container-dir" is missing, then docker creates a new volume.
    --volumes-from="": Mount all volumes from the given container(s)

The volumes commands are complex enough to have their own documentation
in section [*Share Directories via
Volumes*](../../use/working_with_volumes/#volume-def). A developer can
define one or more `VOLUME`s associated with an
image, but only the operator can give access from one container to
another (or from a container to a volume mounted on the host).

### [USER](#id17)

The default user within a container is `root` (id =
0), but if the developer created additional users, those are accessible
too. The developer can set a default user to run the first process with
the `Dockerfile USER` command, but the operator can
override it

    -u="": Username or UID

### [WORKDIR](#id18)

The default working directory for running binaries within a container is
the root directory (`/`), but the developer can set
a different default with the `Dockerfile WORKDIR`
command. The operator can override this with:

    -w="": Working directory inside the container

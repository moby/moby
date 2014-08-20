page_title: Docker Run Reference
page_description: Configure containers at runtime
page_keywords: docker, run, configure, runtime

# Docker Run Reference

**Docker runs processes in isolated containers**. When an operator
executes `docker run`, she starts a process with its own file system,
its own networking, and its own isolated process tree.  The
[*Image*](/terms/image/#image-def) which starts the process may define
defaults related to the binary to run, the networking to expose, and
more, but `docker run` gives final control to the operator who starts
the container from the image. That's the main reason
[*run*](/reference/commandline/cli/#run) has more options than any
other `docker` command.

## General Form

The basic `docker run` command takes this form:

    $ docker run [OPTIONS] IMAGE[:TAG] [COMMAND] [ARG...]

To learn how to interpret the types of `[OPTIONS]`,
see [*Option types*](/reference/commandline/cli/#option-types).

The list of `[OPTIONS]` breaks down into two groups:

1. Settings exclusive to operators, including:
     * Detached or Foreground running,
     * Container Identification,
     * Network settings, and
     * Runtime Constraints on CPU and Memory
     * Privileges and LXC Configuration
2. Settings shared between operators and developers, where operators can
   override defaults developers set in images at build time.

Together, the `docker run [OPTIONS]` give the operator complete control over runtime
behavior, allowing them to override all defaults set by
the developer during `docker build` and nearly all the defaults set by
the Docker runtime itself.

## Operator Exclusive Options

Only the operator (the person executing `docker run`) can set the
following options.

 - [Detached vs Foreground](#detached-vs-foreground)
     - [Detached (-d)](#detached-d)
     - [Foreground](#foreground)
 - [Container Identification](#container-identification)
     - [Name (--name)](#name-name)
     - [PID Equivalent](#pid-equivalent)
 - [Network Settings](#network-settings)
 - [Clean Up (--rm)](#clean-up-rm)
 - [Runtime Constraints on CPU and Memory](#runtime-constraints-on-cpu-and-memory)
 - [Runtime Privilege, Linux Capabilities, and LXC Configuration](#runtime-privilege-linux-capabilities-and-lxc-configuration)

## Detached vs Foreground

When starting a Docker container, you must first decide if you want to
run the container in the background in a "detached" mode or in the
default foreground mode:

    -d=false: Detached mode: Run container in the background, print new container id

### Detached (-d)

In detached mode (`-d=true` or just `-d`), all I/O should be done
through network connections or shared volumes because the container is
no longer listening to the command line where you executed `docker run`.
You can reattach to a detached container with `docker`
[*attach*](/reference/commandline/cli/#attach). If you choose to run a
container in the detached mode, then you cannot use the `--rm` option.

### Foreground

In foreground mode (the default when `-d` is not specified), `docker
run` can start the process in the container and attach the console to
the process's standard input, output, and standard error. It can even
pretend to be a TTY (this is what most command line executables expect)
and pass along signals. All of that is configurable:

    -a=[]           : Attach to `STDIN`, `STDOUT` and/or `STDERR`
    -t=false        : Allocate a pseudo-tty
    --sig-proxy=true: Proxify all received signal to the process (even in non-tty mode)
    -i=false        : Keep STDIN open even if not attached

If you do not specify `-a` then Docker will [attach all standard
streams]( https://github.com/docker/docker/blob/
75a7f4d90cde0295bcfb7213004abce8d4779b75/commands.go#L1797). You can
specify to which of the three standard streams (`STDIN`, `STDOUT`,
`STDERR`) you'd like to connect instead, as in:

    $ docker run -a stdin -a stdout -i -t ubuntu /bin/bash

For interactive processes (like a shell) you will typically want a tty
as well as persistent standard input (`STDIN`), so you'll use `-i -t`
together in most interactive cases.

## Container Identification

### Name (–-name)

The operator can identify a container in three ways:

-   UUID long identifier
    ("f78375b1c487e03c9438c729345e54db9d20cfa2ac1fc3494b6eb60872e74778")
-   UUID short identifier ("f78375b1c487")
-   Name ("evil_ptolemy")

The UUID identifiers come from the Docker daemon, and if you do not
assign a name to the container with `--name` then the daemon will also
generate a random string name too. The name can become a handy way to
add meaning to a container since you can use this name when defining
[*links*](/userguide/dockerlinks/#working-with-links-names) (or any
other place you need to identify a container). This works for both
background and foreground Docker containers.

### PID Equivalent

Finally, to help with automation, you can have Docker write the
container ID out to a file of your choosing. This is similar to how some
programs might write out their process ID to a file (you've seen them as
PID files):

    --cidfile="": Write the container ID to the file
    
### Image[:tag]

While not strictly a means of identifying a container, you can specify a version of an
image you'd like to run the container with by adding `image[:tag]` to the command. For
example, `docker run ubuntu:14.04`.

## Network Settings

    --dns=[]        : Set custom dns servers for the container
    --net="bridge"  : Set the Network mode for the container
                                 'bridge': creates a new network stack for the container on the docker bridge
                                 'none': no networking for this container
                                 'container:<name|id>': reuses another container network stack
                                 'host': use the host network stack inside the container

By default, all containers have networking enabled and they can make any
outgoing connections. The operator can completely disable networking
with `docker run --net none` which disables all incoming and outgoing
networking. In cases like this, you would perform I/O through files or
`STDIN` and `STDOUT` only.

Your container will use the same DNS servers as the host by default, but
you can override this with `--dns`.

Supported networking modes are:

* none - no networking in the container
* bridge - (default) connect the container to the bridge via veth interfaces
* host - use the host's network stack inside the container.  Note: This gives the container full access to local system services such as D-bus and is therefore considered insecure.
* container - use another container's network stack

#### Mode: none

With the networking mode set to `none` a container will not have a
access to any external routes.  The container will still have a
`loopback` interface enabled in the container but it does not have any
routes to external traffic.

#### Mode: bridge

With the networking mode set to `bridge` a container will use docker's
default networking setup.  A bridge is setup on the host, commonly named
`docker0`, and a pair of `veth` interfaces will be created for the
container.  One side of the `veth` pair will remain on the host attached
to the bridge while the other side of the pair will be placed inside the
container's namespaces in addition to the `loopback` interface.  An IP
address will be allocated for containers on the bridge's network and
traffic will be routed though this bridge to the container.

#### Mode: host

With the networking mode set to `host` a container will share the host's
network stack and all interfaces from the host will be available to the
container.  The container's hostname will match the hostname on the host
system.  Publishing ports and linking to other containers will not work
when sharing the host's network stack.

#### Mode: container

With the networking mode set to `container` a container will share the
network stack of another container.  The other container's name must be
provided in the format of `--net container:<name|id>`.

Example running a Redis container with Redis binding to `localhost` then
running the `redis-cli` command and connecting to the Redis server over the
`localhost` interface.

    $ docker run -d --name redis example/redis --bind 127.0.0.1
    $ # use the redis container's network stack to access localhost
    $ docker run --rm -ti --net container:redis example/redis-cli -h 127.0.0.1

## Clean Up (–-rm)

By default a container's file system persists even after the container
exits. This makes debugging a lot easier (since you can inspect the
final state) and you retain all your data by default. But if you are
running short-term **foreground** processes, these container file
systems can really pile up. If instead you'd like Docker to
**automatically clean up the container and remove the file system when
the container exits**, you can add the `--rm` flag:

    --rm=false: Automatically remove the container when it exits (incompatible with -d)

## Runtime Constraints on CPU and Memory

The operator can also adjust the performance parameters of the
container:

    -m="": Memory limit (format: <number><optional unit>, where unit = b, k, m or g)
    -c=0 : CPU shares (relative weight)

The operator can constrain the memory available to a container easily
with `docker run -m`. If the host supports swap memory, then the `-m`
memory setting can be larger than physical RAM.

Similarly the operator can increase the priority of this container with
the `-c` option. By default, all containers run at the same priority and
get the same proportion of CPU cycles, but you can tell the kernel to
give more shares of CPU time to one or more containers when you start
them via Docker.

## Runtime Privilege, Linux Capabilities, and LXC Configuration

    --cap-add: Add Linux capabilities
    --cap-drop: Drop Linux capabilities
    --privileged=false: Give extended privileges to this container
    --lxc-conf=[]: (lxc exec-driver only) Add custom lxc options --lxc-conf="lxc.cgroup.cpuset.cpus = 0,1"

By default, Docker containers are "unprivileged" and cannot, for
example, run a Docker daemon inside a Docker container. This is because
by default a container is not allowed to access any devices, but a
"privileged" container is given access to all devices (see [lxc-template.go](
https://github.com/docker/docker/blob/master/daemon/execdriver/lxc/lxc_template.go)
and documentation on [cgroups devices](
https://www.kernel.org/doc/Documentation/cgroups/devices.txt)).

When the operator executes `docker run --privileged`, Docker will enable
to access to all devices on the host as well as set some configuration
in AppArmor to allow the container nearly all the same access to the
host as processes running outside containers on the host. Additional
information about running with `--privileged` is available on the
[Docker Blog](http://blog.docker.com/2013/09/docker-can-now-run-within-docker/).

In addition to `--privileged`, the operator can have fine grain control over the
capabilities using `--cap-add` and `--cap-drop`. By default, Docker has a default
list of capabilities that are kept. Both flags support the value `all`, so if the
operator wants to have all capabilities but `MKNOD` they could use:

    $ docker run --cap-add=ALL --cap-drop=MKNOD ...

For interacting with the network stack, instead of using `--privileged` they
should use `--cap-add=NET_ADMIN` to modify the network interfaces.

If the Docker daemon was started using the `lxc` exec-driver
(`docker -d --exec-driver=lxc`) then the operator can also specify LXC options
using one or more `--lxc-conf` parameters. These can be new parameters or
override existing parameters from the [lxc-template.go](
https://github.com/docker/docker/blob/master/daemon/execdriver/lxc/lxc_template.go).
Note that in the future, a given host's docker daemon may not use LXC, so this
is an implementation-specific configuration meant for operators already
familiar with using LXC directly.

## Overriding Dockerfile Image Defaults

When a developer builds an image from a [*Dockerfile*](/reference/builder/#dockerbuilder)
or when she commits it, the developer can set a number of default parameters
that take effect when the image starts up as a container.

Four of the Dockerfile commands cannot be overridden at runtime: `FROM`,
`MAINTAINER`, `RUN`, and `ADD`. Everything else has a corresponding override
in `docker run`. We'll go through what the developer might have set in each
Dockerfile instruction and how the operator can override that setting.

 - [CMD (Default Command or Options)](#cmd-default-command-or-options)
 - [ENTRYPOINT (Default Command to Execute at Runtime)](
    #entrypoint-default-command-to-execute-at-runtime)
 - [EXPOSE (Incoming Ports)](#expose-incoming-ports)
 - [ENV (Environment Variables)](#env-environment-variables)
 - [VOLUME (Shared Filesystems)](#volume-shared-filesystems)
 - [USER](#user)
 - [WORKDIR](#workdir)

## CMD (Default Command or Options)

Recall the optional `COMMAND` in the Docker
commandline:

    $ docker run [OPTIONS] IMAGE[:TAG] [COMMAND] [ARG...]

This command is optional because the person who created the `IMAGE` may
have already provided a default `COMMAND` using the Dockerfile `CMD`
instruction. As the operator (the person running a container from the
image), you can override that `CMD` instruction just by specifying a new
`COMMAND`.

If the image also specifies an `ENTRYPOINT` then the `CMD` or `COMMAND`
get appended as arguments to the `ENTRYPOINT`.

## ENTRYPOINT (Default Command to Execute at Runtime)

    --entrypoint="": Overwrite the default entrypoint set by the image

The `ENTRYPOINT` of an image is similar to a `COMMAND` because it
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

    $ docker run -i -t --entrypoint /bin/bash example/redis

or two examples of how to pass more parameters to that ENTRYPOINT:

    $ docker run -i -t --entrypoint /bin/bash example/redis -c ls -l
    $ docker run -i -t --entrypoint /usr/bin/redis-cli example/redis --help

## EXPOSE (Incoming Ports)

The Dockerfile doesn't give much control over networking, only providing
the `EXPOSE` instruction to give a hint to the operator about what
incoming ports might provide services. The following options work with
or override the Dockerfile's exposed defaults:

    --expose=[]: Expose a port from the container
                without publishing it to your host
    -P=false   : Publish all exposed ports to the host interfaces
    -p=[]      : Publish a container᾿s port to the host (format:
                 ip:hostPort:containerPort | ip::containerPort |
                 hostPort:containerPort)
                 (use 'docker port' to see the actual mapping)
    --link=""  : Add link to another container (name:alias)

As mentioned previously, `EXPOSE` (and `--expose`) make a port available
**in** a container for incoming connections. The port number on the
inside of the container (where the service listens) does not need to be
the same number as the port exposed on the outside of the container
(where clients connect), so inside the container you might have an HTTP
service listening on port 80 (and so you `EXPOSE 80` in the Dockerfile),
but outside the container the port might be 42800.

To help a new client container reach the server container's internal
port operator `--expose`'d by the operator or `EXPOSE`'d by the
developer, the operator has three choices: start the server container
with `-P` or `-p,` or start the client container with `--link`.

If the operator uses `-P` or `-p` then Docker will make the exposed port
accessible on the host and the ports will be available to any client
that can reach the host. To find the map between the host ports and the
exposed ports, use `docker port`)

If the operator uses `--link` when starting the new client container,
then the client container can access the exposed port via a private
networking interface.  Docker will set some environment variables in the
client container to help indicate which interface and port to use.

## ENV (Environment Variables)

The operator can **set any environment variable** in the container by
using one or more `-e` flags, even overriding those already defined by
the developer with a Dockerfile `ENV`:

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

`--link name:alias` also sets environment variables, using the *alias* string to
define environment variables within the container that give the IP and PORT
information for connecting to the service container. Let's imagine we have a
container running Redis:

    # Start the service container, named redis-name
    $ docker run -d --name redis-name dockerfiles/redis
    4241164edf6f5aca5b0e9e4c9eccd899b0b8080c64c0cd26efe02166c73208f3

    # The redis-name container exposed port 6379
    $ docker ps
    CONTAINER ID        IMAGE                      COMMAND                CREATED             STATUS              PORTS               NAMES
    4241164edf6f        $ dockerfiles/redis:latest   /redis-stable/src/re   5 seconds ago       Up 4 seconds        6379/tcp            redis-name

    # Note that there are no public ports exposed since we didn᾿t use -p or -P
    $ docker port 4241164edf6f 6379
    2014/01/25 00:55:38 Error: No public port '6379' published for 4241164edf6f

Yet we can get information about the Redis container's exposed ports
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

And we can use that information to connect from another container as a client:

    $ docker run -i -t --rm --link redis-name:redis_alias --entrypoint /bin/bash dockerfiles/redis -c '/redis-stable/src/redis-cli -h $REDIS_ALIAS_PORT_6379_TCP_ADDR -p $REDIS_ALIAS_PORT_6379_TCP_PORT'
    172.17.0.32:6379>

Docker will also map the private IP address to the alias of a linked
container by inserting an entry into `/etc/hosts`.  You can use this
mechanism to communicate with a linked container by its alias:

    $ docker run -d --name servicename busybox sleep 30
    $ docker run -i -t --link servicename:servicealias busybox ping -c 1 servicealias

## VOLUME (Shared Filesystems)

    -v=[]: Create a bind mount with: [host-dir]:[container-dir]:[rw|ro].
           If "container-dir" is missing, then docker creates a new volume.
    --volumes-from="": Mount all volumes from the given container(s)

The volumes commands are complex enough to have their own documentation
in section [*Managing data in 
containers*](/userguide/dockervolumes/#volume-def). A developer can define
one or more `VOLUME`'s associated with an image, but only the operator
can give access from one container to another (or from a container to a
volume mounted on the host).

## USER

The default user within a container is `root` (id = 0), but if the
developer created additional users, those are accessible too. The
developer can set a default user to run the first process with the
Dockerfile `USER` instruction, but the operator can override it:

    -u="": Username or UID

> **Note:** if you pass numeric uid, it must be in range 0-2147483647.

## WORKDIR

The default working directory for running binaries within a container is the
root directory (`/`), but the developer can set a different default with the
Dockerfile `WORKDIR` command. The operator can override this with:

    -w="": Working directory inside the container

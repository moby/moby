page_title: Docker run reference
page_description: Configure containers at runtime
page_keywords: docker, run, configure, runtime

# Docker run reference

**Docker runs processes in isolated containers**. When an operator
executes `docker run`, she starts a process with its own file system,
its own networking, and its own isolated process tree.  The
[*Image*](/terms/image/#image-def) which starts the process may define
defaults related to the binary to run, the networking to expose, and
more, but `docker run` gives final control to the operator who starts
the container from the image. That's the main reason
[*run*](/reference/commandline/cli/#run) has more options than any
other `docker` command.

## General form

The basic `docker run` command takes this form:

    $ sudo docker run [OPTIONS] IMAGE[:TAG] [COMMAND] [ARG...]

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

## Operator exclusive options

Only the operator (the person executing `docker run`) can set the
following options.

 - [Detached vs Foreground](#detached-vs-foreground)
     - [Detached (-d)](#detached-d)
     - [Foreground](#foreground)
 - [Container Identification](#container-identification)
     - [Name (--name)](#name-name)
     - [PID Equivalent](#pid-equivalent)
 - [IPC Settings](#ipc-settings)
 - [Network Settings](#network-settings)
 - [Clean Up (--rm)](#clean-up-rm)
 - [Runtime Constraints on CPU and Memory](#runtime-constraints-on-cpu-and-memory)
 - [Runtime Privilege, Linux Capabilities, and LXC Configuration](#runtime-privilege-linux-capabilities-and-lxc-configuration)

## Detached vs foreground

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
    --sig-proxy=true: Proxify all received signal to the process (non-TTY mode only)
    -i=false        : Keep STDIN open even if not attached

If you do not specify `-a` then Docker will [attach all standard
streams]( https://github.com/docker/docker/blob/
75a7f4d90cde0295bcfb7213004abce8d4779b75/commands.go#L1797). You can
specify to which of the three standard streams (`STDIN`, `STDOUT`,
`STDERR`) you'd like to connect instead, as in:

    $ sudo docker run -a stdin -a stdout -i -t ubuntu /bin/bash

For interactive processes (like a shell), you must use `-i -t` together in
order to allocate a tty for the container process. Specifying `-t` is however
forbidden when the client standard output is redirected or pipe, such as in:
`echo test | docker run -i busybox cat`.

## Container identification

### Name (--name)

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

### PID equivalent

Finally, to help with automation, you can have Docker write the
container ID out to a file of your choosing. This is similar to how some
programs might write out their process ID to a file (you've seen them as
PID files):

    --cidfile="": Write the container ID to the file
    
### Image[:tag]

While not strictly a means of identifying a container, you can specify a version of an
image you'd like to run the container with by adding `image[:tag]` to the command. For
example, `docker run ubuntu:14.04`.

## IPC Settings
    --ipc=""  : Set the IPC mode for the container,
                                 'container:<name|id>': reuses another container's IPC namespace
                                 'host': use the host's IPC namespace inside the container
By default, all containers have the IPC namespace enabled 

IPC (POSIX/SysV IPC) namespace provides separation of named shared memory segments, semaphores and message queues.  

Shared memory segments are used to accelerate inter-process communication at
memory speed, rather than through pipes or through the network stack. Shared
memory is commonly used by databases and custom-built (typically C/OpenMPI, 
C++/using boost libraries) high performance applications for scientific
computing and financial services industries. If these types of applications
are broken into multiple containers, you might need to share the IPC mechanisms
of the containers.

## Network settings

    --dns=[]         : Set custom dns servers for the container
    --net="bridge"   : Set the Network mode for the container
                                  'bridge': creates a new network stack for the container on the docker bridge
                                  'none': no networking for this container
                                  'container:<name|id>': reuses another container network stack
                                  'host': use the host network stack inside the container
    --add-host=""    : Add a line to /etc/hosts (host:IP)
    --mac-address="" : Sets the container's Ethernet device's MAC address

By default, all containers have networking enabled and they can make any
outgoing connections. The operator can completely disable networking
with `docker run --net none` which disables all incoming and outgoing
networking. In cases like this, you would perform I/O through files or
`STDIN` and `STDOUT` only.

Your container will use the same DNS servers as the host by default, but
you can override this with `--dns`.

By default a random MAC is generated. You can set the container's MAC address
explicitly by providing a MAC via the `--mac-address` parameter (format:
`12:34:56:78:9a:bc`).

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

    $ sudo docker run -d --name redis example/redis --bind 127.0.0.1
    $ # use the redis container's network stack to access localhost
    $ sudo docker run --rm -ti --net container:redis example/redis-cli -h 127.0.0.1

### Managing /etc/hosts

Your container will have lines in `/etc/hosts` which define the hostname of the
container itself as well as `localhost` and a few other common things.  The
`--add-host` flag can be used to add additional lines to `/etc/hosts`.  

    $ /docker run -ti --add-host db-static:86.75.30.9 ubuntu cat /etc/hosts
    172.17.0.22     09d03f76bf2c
    fe00::0         ip6-localnet
    ff00::0         ip6-mcastprefix
    ff02::1         ip6-allnodes
    ff02::2         ip6-allrouters
    127.0.0.1       localhost
    ::1	            localhost ip6-localhost ip6-loopback
    86.75.30.9      db-static

## Clean up (--rm)

By default a container's file system persists even after the container
exits. This makes debugging a lot easier (since you can inspect the
final state) and you retain all your data by default. But if you are
running short-term **foreground** processes, these container file
systems can really pile up. If instead you'd like Docker to
**automatically clean up the container and remove the file system when
the container exits**, you can add the `--rm` flag:

    --rm=false: Automatically remove the container when it exits (incompatible with -d)

## Security configuration
    --security-opt="label:user:USER"   : Set the label user for the container
    --security-opt="label:role:ROLE"   : Set the label role for the container
    --security-opt="label:type:TYPE"   : Set the label type for the container
    --security-opt="label:level:LEVEL" : Set the label level for the container
    --security-opt="label:disable"     : Turn off label confinement for the container
    --secutity-opt="apparmor:PROFILE"  : Set the apparmor profile to be applied 
                                         to the container

You can override the default labeling scheme for each container by specifying
the `--security-opt` flag. For example, you can specify the MCS/MLS level, a
requirement for MLS systems. Specifying the level in the following command
allows you to share the same content between containers.

    # docker run --security-opt label:level:s0:c100,c200 -i -t fedora bash

An MLS example might be:

    # docker run --security-opt label:level:TopSecret -i -t rhel7 bash

To disable the security labeling for this container versus running with the
`--permissive` flag, use the following command:

    # docker run --security-opt label:disable -i -t fedora bash

If you want a tighter security policy on the processes within a container,
you can specify an alternate type for the container. You could run a container
that is only allowed to listen on Apache ports by executing the following
command:

    # docker run --security-opt label:type:svirt_apache_t -i -t centos bash

Note:

You would have to write policy defining a `svirt_apache_t` type.

## Runtime constraints on CPU and memory

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

The flag `-c` or `--cpu-shares` with value 0 indicates that the running
container has access to all 1024 (default) CPU shares. However, this value
can be modified to run a container with a different priority or different
proportion of CPU cycles.

E.g., If we start three {C0, C1, C2} containers with default values
(`-c` OR `--cpu-shares` = 0) and one {C3} with (`-c` or `--cpu-shares`=512)
then C0, C1, and C2 would have access to 100% CPU shares (1024) and C3 would
only have access to 50% CPU shares (512). In the context of a time-sliced OS
with time quantum set as 100 milliseconds, containers C0, C1, and C2 will run
for full-time quantum, and container C3 will run for half-time quantum i.e 50
milliseconds.

## Runtime privilege, Linux capabilities, and LXC configuration

    --cap-add: Add Linux capabilities
    --cap-drop: Drop Linux capabilities
    --privileged=false: Give extended privileges to this container
    --device=[]: Allows you to run devices inside the container without the --privileged flag.
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
in AppArmor or SELinux to allow the container nearly all the same access to the
host as processes running outside containers on the host. Additional
information about running with `--privileged` is available on the
[Docker Blog](http://blog.docker.com/2013/09/docker-can-now-run-within-docker/).

If you want to limit access to a specific device or devices you can use
the `--device` flag. It allows you to specify one or more devices that
will be accessible within the container.

    $ sudo docker run --device=/dev/snd:/dev/snd ...

By default, the container will be able to `read`, `write`, and `mknod` these devices.
This can be overridden using a third `:rwm` set of options to each `--device` flag:


```
	$ sudo docker run --device=/dev/sda:/dev/xvdc --rm -it ubuntu fdisk  /dev/xvdc

	Command (m for help): q
	$ sudo docker run --device=/dev/sda:/dev/xvdc:r --rm -it ubuntu fdisk  /dev/xvdc
	You will not be able to write the partition table.

	Command (m for help): q

	$ sudo docker run --device=/dev/sda:/dev/xvdc:w --rm -it ubuntu fdisk  /dev/xvdc
        crash....

	$ sudo docker run --device=/dev/sda:/dev/xvdc:m --rm -it ubuntu fdisk  /dev/xvdc
	fdisk: unable to open /dev/xvdc: Operation not permitted
```

In addition to `--privileged`, the operator can have fine grain control over the
capabilities using `--cap-add` and `--cap-drop`. By default, Docker has a default
list of capabilities that are kept. Both flags support the value `all`, so if the
operator wants to have all capabilities but `MKNOD` they could use:

    $ sudo docker run --cap-add=ALL --cap-drop=MKNOD ...

For interacting with the network stack, instead of using `--privileged` they
should use `--cap-add=NET_ADMIN` to modify the network interfaces.

    $ docker run -t -i --rm  ubuntu:14.04 ip link add dummy0 type dummy
    RTNETLINK answers: Operation not permitted
    $ docker run -t -i --rm --cap-add=NET_ADMIN ubuntu:14.04 ip link add dummy0 type dummy

To mount a FUSE based filesystem, you need to combine both `--cap-add` and
`--device`:

    $ docker run --rm -it --cap-add SYS_ADMIN sshfs sshfs sven@10.10.10.20:/home/sven /mnt
    fuse: failed to open /dev/fuse: Operation not permitted
    $ docker run --rm -it --device /dev/fuse sshfs sshfs sven@10.10.10.20:/home/sven /mnt
    fusermount: mount failed: Operation not permitted
    $ docker run --rm -it --cap-add SYS_ADMIN --device /dev/fuse sshfs
    # sshfs sven@10.10.10.20:/home/sven /mnt
    The authenticity of host '10.10.10.20 (10.10.10.20)' can't be established.
    ECDSA key fingerprint is 25:34:85:75:25:b0:17:46:05:19:04:93:b5:dd:5f:c6.
    Are you sure you want to continue connecting (yes/no)? yes
    sven@10.10.10.20's password:
    root@30aa0cfaf1b5:/# ls -la /mnt/src/docker
    total 1516
    drwxrwxr-x 1 1000 1000   4096 Dec  4 06:08 .
    drwxrwxr-x 1 1000 1000   4096 Dec  4 11:46 ..
    -rw-rw-r-- 1 1000 1000     16 Oct  8 00:09 .dockerignore
    -rwxrwxr-x 1 1000 1000    464 Oct  8 00:09 .drone.yml
    drwxrwxr-x 1 1000 1000   4096 Dec  4 06:11 .git
    -rw-rw-r-- 1 1000 1000    461 Dec  4 06:08 .gitignore
    ....


If the Docker daemon was started using the `lxc` exec-driver
(`docker -d --exec-driver=lxc`) then the operator can also specify LXC options
using one or more `--lxc-conf` parameters. These can be new parameters or
override existing parameters from the [lxc-template.go](
https://github.com/docker/docker/blob/master/daemon/execdriver/lxc/lxc_template.go).
Note that in the future, a given host's docker daemon may not use LXC, so this
is an implementation-specific configuration meant for operators already
familiar with using LXC directly.

> **Note:**
> If you use `--lxc-conf` to modify a container's configuration which is also
> managed by the Docker daemon, then the Docker daemon will not know about this
> modification, and you will need to manage any conflicts yourself. For example,
> you can use `--lxc-conf` to set a container's IP address, but this will not be
> reflected in the `/etc/hosts` file.

## Overriding Dockerfile image defaults

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

## CMD (default command or options)

Recall the optional `COMMAND` in the Docker
commandline:

    $ sudo docker run [OPTIONS] IMAGE[:TAG] [COMMAND] [ARG...]

This command is optional because the person who created the `IMAGE` may
have already provided a default `COMMAND` using the Dockerfile `CMD`
instruction. As the operator (the person running a container from the
image), you can override that `CMD` instruction just by specifying a new
`COMMAND`.

If the image also specifies an `ENTRYPOINT` then the `CMD` or `COMMAND`
get appended as arguments to the `ENTRYPOINT`.

## ENTRYPOINT (default command to execute at runtime)

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

    $ sudo docker run -i -t --entrypoint /bin/bash example/redis

or two examples of how to pass more parameters to that ENTRYPOINT:

    $ sudo docker run -i -t --entrypoint /bin/bash example/redis -c ls -l
    $ sudo docker run -i -t --entrypoint /usr/bin/redis-cli example/redis --help

## EXPOSE (incoming ports)

The Dockerfile doesn't give much control over networking, only providing
the `EXPOSE` instruction to give a hint to the operator about what
incoming ports might provide services. The following options work with
or override the Dockerfile's exposed defaults:

    --expose=[]: Expose a port or a range of ports from the container
                without publishing it to your host
    -P=false   : Publish all exposed ports to the host interfaces
    -p=[]      : Publish a container᾿s port to the host (format:
                 ip:hostPort:containerPort | ip::containerPort |
                 hostPort:containerPort | containerPort)
                 (use 'docker port' to see the actual mapping)
    --link=""  : Add link to another container (name:alias)

As mentioned previously, `EXPOSE` (and `--expose`) makes ports available
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
that can reach the host. When using `-P`, Docker will bind the exposed 
ports to a random port on the host between 49153 and 65535. To find the
mapping between the host ports and the exposed ports, use `docker port`.

If the operator uses `--link` when starting the new client container,
then the client container can access the exposed port via a private
networking interface.  Docker will set some environment variables in the
client container to help indicate which interface and port to use.

## ENV (environment variables)

When a new container is created, Docker will set the following environment
variables automatically:

<table width=100%>
 <tr style="background-color:#C0C0C0">
  <td> <b>Variable</b> </td>
  <td style="padding-left:10px"> <b>Value</b> </td>
 </tr>
 <tr>
  <td> <code>HOME</code> </td>
  <td style="padding-left:10px">
    Set based on the value of <code>USER</code>
  </td>
 </tr>
 <tr style="background-color:#E8E8E8">
  <td valign=top> <code>HOSTNAME</code> </td>
  <td style="padding-left:10px"> 
    The hostname associated with the container
  </td>
 </tr>
 <tr>
  <td valign=top> <code>PATH</code> </td>
  <td style="padding-left:10px"> 
    Includes popular directories, such as :<br>
    <code>/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin</code>
  </td>
 <tr style="background-color:#E8E8E8">
  <td valign=top> <code>TERM</code> </td>
  <td style="padding-left:10px"> 
    <code>xterm</code> if the container is allocated a psuedo-TTY 
  </td>
 </tr>
</table>

The container may also include environment variables defined
as a result of the container being linked with another container. See
the [*Container Links*](/userguide/dockerlinks/#container-linking)
section for more details.

Additionally, the operator can **set any environment variable** in the 
container by using one or more `-e` flags, even overriding those mentioned 
above, or already defined by the developer with a Dockerfile `ENV`:

    $ sudo docker run -e "deep=purple" --rm ubuntu /bin/bash -c export
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
    $ sudo docker run -d --name redis-name dockerfiles/redis
    4241164edf6f5aca5b0e9e4c9eccd899b0b8080c64c0cd26efe02166c73208f3

    # The redis-name container exposed port 6379
    $ sudo docker ps
    CONTAINER ID        IMAGE                      COMMAND                CREATED             STATUS              PORTS               NAMES
    4241164edf6f        $ dockerfiles/redis:latest   /redis-stable/src/re   5 seconds ago       Up 4 seconds        6379/tcp            redis-name

    # Note that there are no public ports exposed since we didn᾿t use -p or -P
    $ sudo docker port 4241164edf6f 6379
    2014/01/25 00:55:38 Error: No public port '6379' published for 4241164edf6f

Yet we can get information about the Redis container's exposed ports
with `--link`. Choose an alias that will form a
valid environment variable!

    $ sudo docker run --rm --link redis-name:redis_alias --entrypoint /bin/bash dockerfiles/redis -c export
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

    $ sudo docker run -i -t --rm --link redis-name:redis_alias --entrypoint /bin/bash dockerfiles/redis -c '/redis-stable/src/redis-cli -h $REDIS_ALIAS_PORT_6379_TCP_ADDR -p $REDIS_ALIAS_PORT_6379_TCP_PORT'
    172.17.0.32:6379>

Docker will also map the private IP address to the alias of a linked
container by inserting an entry into `/etc/hosts`.  You can use this
mechanism to communicate with a linked container by its alias:

    $ sudo docker run -d --name servicename busybox sleep 30
    $ sudo docker run -i -t --link servicename:servicealias busybox ping -c 1 servicealias

If you restart the source container (`servicename` in this case), the recipient
container's `/etc/hosts` entry will be automatically updated.

## VOLUME (shared filesystems)

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

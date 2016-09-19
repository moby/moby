---
title: "Docker run reference"
description: "Configure containers at runtime"
keywords: "docker, run, configure, runtime"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# Running Docker containers

The `docker run` command is a central Docker command. In short, it combines
`docker pull` (if necessary), `docker create` (if necessary), and `docker start`.
First, it checks to see if you have the image locally and pulls it from Docker
Hub or a private repository if necessary. Next, it creates a container from the
image. Finally, it starts the container. The following two examples are
equivalent.

```bash
$ docker pull ubuntu

$ docker create -it --name="my_ubuntu" ubuntu

$ docker start my_ubuntu
```

```bash
$ docker run -it --name="my_ubuntu" ubuntu
```

The `run` command supports a lot of flags and options which allow you to
override or augment the container's default runtime behavior, set environment
variables within the container, control aspects of storage, networking, and
security, and other types of functionality.

**This topic topic has a lot of information, but it only skims the surface of
what the ways you can control containers at runtime.** See the [Docker
command-line reference](commandline/run.md) or the output of the
`docker help run` command for all of the possibilities.

This topic is organized so that the most universally useful information is near
the top. More expert-level options are discussed nearer the bottom.

## `docker run` flag syntax

Many `docker run` flags have a long and short form. Short-form flags start with
a single hyphen character (`-`), and long-form flags start with a double hyphen
character (`--`). For instance, `-i` is equivalent to `--interactive`. The
following syntax guidelines apply.

- When a short-form flag takes an option, the option is separated from the flag
  by a space.

- Short-form flags which do not take an argument are boolean, and are `true` if
  present. These boolean flags can be combined. For instance, `-i -t` is
  equivalent to `-it` or `--interactive --tty`.

- When a long-form flag takes an argument, the argument is separated from the
  flag by either a space or an equals sign (`=`). Surrounding option strings
  with quotes is optional unless they contain spaces. These instructions use the
  `=` separator and always surround flag arguments with double quotes.

- A few long-form flags take an argument that is a string of one or more
  key-value pairs instead of a single argument. For instance, the `--mount` flag
  has several optional arguments. An example of this type of syntax is
  `--mount type=volume,target=/mnt/my_volume`.

## Understanding the basics

### Running your first command

In its most basic form, the `docker run` command requires an image name and may
require a command, if the Dockerfile does not specify a default command. In the
command below, `alpine` is the image, and `pwd` is the command. You can specify
an image version by appending it to the image name, separated by a colon (for
instance, `alpine:1.10.3`). You can specify a _digest_, which is the image's
ID in SHA1 format, by appending it to the image name, separated by an `@` symbol.
If you do not specify an image version or digest, the latest version is used.

```bash
$ docker run alpine "pwd"

/
```

This command returns the present working directory (`/`) and exits immediately,
because the command has finished.

### Removing the container when it stops

By default, containers are not removed when they exit. Generally, containers do
not use much space, but when you are testing, you may create a lot of containers
you don't want to keep around forever. Use `docker ps -a` to list all running
and stopped containers. The last column is the container name. Use
`docker rm "<container_name>"` to remove a stopped container.

A full discussion of either of the `docker ps` and `docker rm` commands is out
of scope for this topic. Refer to the reference topics for
[`docker ps`](commandline/ps.md) and [`docker rm`](commandline/rm.md)
for more details.

If you pass the `--rm` flag when running a container, the container will be
removed when it exits. This example modifies the previous one to remove the
container on exit.

```bash
$ docker run --rm alpine "pwd"
```

> **Note**: When you set the `--rm` flag, Docker also removes any anonymous
(unnamed) volumes associated with the container, unless they are also used by
other containers. Using an anonymous volume in a second container essentially
converts it into a named volume. Explicitly named volumes are not removed
by `docker run` or `docker rm`.


### Running interactively

If you want to run a single command and get its output, or if you want to run a
long-running process like a web server, you do not need to interact with the
container. However, if you want to run commands within the container or test
things out, you can use the `-i` or `--interactive` flag to run the container
interactively. If you are running a command that takes input and sends output,
like a shell, you also need the `-t` or `--tty` flag, which causes Docker to use
your local terminal's input and output so that you can type commands and see
output.

>The `--interactive` and `--tty` flags are often run together as `-it`. They are
often combined with `--rm`, in the form `--rm -it`. This means the container is
interactive, attaches to a TTY, and will be removed when it exits.

This example starts an `ubuntu` container interactively, running the `/bin/bash`
shell.

```bash
$ docker run --rm -it ubuntu "/bin/bash"
```

Your command prompt will change to something like `root@476da51f112c:/#`, which
indicates that you are the `root` user on a container with the ID `476da51f112c`,
and that your working directory is `/`. You can now run any Linux commands that
are available in the `ubuntu` container, and you can even install more commands
using Ubuntu command-line tools. However, changes that you make within the container
do not persist when the container is stopped.

If you do not specify `-a` then Docker will [attach to both stdout and stderr
]( https://github.com/docker/docker/blob/4118e0c9eebda2412a09ae66e90c34b85fae3275/runconfig/opts/parse.go#L267).
You can specify to which of the three standard streams (`STDIN`, `STDOUT`,
`STDERR`) you'd like to connect instead. The following example connects to
`STDIN` and `STDOUT` but not `STDERR`:

```bash
$ docker run -a stdin -a stdout -i -t ubuntu /bin/bash
```

>**Warning**: Don't try to stop your container using operating system commands
like `shutdown` or `restart`. Your container is not a virtual machine, but is
running within the host machine's kernel, and these commands will not work as
expected. If you are running as a normal user, they will produce an error. If
you are running the container in privileged mode, the command may actually
shutdown or restart your host machine. To stop your container, use the
`docker stop` command from the host machine. See
[Privileged mode](#privileged-mode) for more information.

### Running in detached mode

By default, containers run in the foreground. You can run the container in
_detached_ mode instead, using the `-d` or `--detached` flag.

>**Note**: You cannot use the `-d` flag in combination with `--rm`,
`--interactive`, or `--tty` flags.

The following command starts an `ubuntu` container and runs the `/bin/bash`
command, but uses detached mode. The command returns the container ID, and you
are returned to the command prompt.

```bash
$ docker run -d -it ubuntu "/bin/bash"

8f316ce8a6c62854ec1bc7e1f8722f4f6866bccb0ff76240ebc7cc777bdd4867
```

To attach to the container, use `docker attach` with either the container ID or
the container name, gleaned from `docker ps`.

```bash
$ docker attach 8f316ce8a6c62854ec1bc7e1f8722f4f6866bccb0ff76240ebc7cc777bdd4867
```

Even though you are attached, you may not see a prompt, because the
prompt was already shown when the container started. In this case, just press
`ENTER` a second time, and you will see the prompt. You can attach to a
container multiple times simultaneously.

If you type `exit` while you are attached to the `ubuntu` container, the container
will exit. If you want to detach without stopping the container, you need to use
the _escape sequence_, which defaults to `CTRL+p CTRL+q`. This is configurable.

For more information about attaching and detaching from containers, including
configuring the detach sequence and requiring a key to attach, see the
[`docker attach`](commandline/attach.md) command reference.

### Naming your container

Each container has a UUID and a name. By default, the name is a randomly-chosen
string which consists of an adjective and a noun separated by an underscore, and
is guaranteed to be unique on a given Docker host. The UUID has a short form and
a long form. To see the UUIDs and names of your containers, run `docker ps -a`
and review the first and last column of output.

#### Example container UUID and name
| Identifier type      | Example value  |
|----------------------|----------------|
| UUID long identifier | `f78375b1c487e03c9438c729345e54db9d20cfa2ac1fc3494b6eb60872e74778` |
| UUID short identifier| `f78375b1c487` |
| Name                 | `evil_ptolemy` |

You can use the name or the short UUID anywhere you could use the full UUID ID
in Docker commands. The container's name also resolves to the container's IP
address when containers are connected to the same Docker user-defined network.

You can assign a name to a container at runtime using the `--name` flag.
Container names may consist of mixed case letters, numerals, underscores,
hyphens, and periods. The following example starts an `ubuntu` container named
`acme_container` in the foreground, in interactive mode.

```bash
$ docker run --rm -it --name="acme_container" ubuntu "/bin/bash"
```

Open another terminal and issue `docker ps`. The last column shows the
container's name. In commands like `docker inspect` or `docker rm`, you can
refer to the container by name. Try using `docker inspect "acme_container"`.

Go back to the original terminal and type `exit` to stop and remove your
container.

### Overriding the image's command

When an image is created, the `CMD` and `ENTRYPOINT` are optional. If they are
specified in the Dockerfile, the entrypoint and command will be concatenated
together and run. If no entrypoint is specified but a command is specified, the
command is executed. The following command runs the `ubuntu` image, whose default
command is `/bin/bash`:

```bash
$ docker run --rm -it ubuntu

root@a394c2d06238:/#
```

Type `exit` to stop and remove the container. Next, run the following command to
override the `CMD` to run `/bin/ls`:

```bash
$ docker run --rm -it ubuntu "/bin/ls"

bin   dev  home  lib64 	mnt  proc  run 	 srv  tmp  var
boot  etc  lib 	 media 	opt  root  sbin  sys  usr
```

The `/bin/ls` command is executed and the container exits immediately. Next,
specify both an entrypoint and a command:

```bash
$ docker run --rm -it --entrypoint="/bin/ls" ubuntu "-l"
total 64
drwxr-xr-x  2 root root 4096 Aug  9 16:25 bin
drwxr-xr-x  2 root root 4096 Apr 12 20:14 boot
drwxr-xr-x  5 root root  380 Sep 20 18:07 dev
drwxr-xr-x 45 root root 4096 Sep 20 18:07 etc
...
<output truncated>
```
The  command `ls -l` is run and the container exits and is removed. You could
accomplish the same thing with multiple commands and no entrypoint:

```bash
$ docker run --rm -it ubuntu "/bin/ls" "-l"
```

>**Note**: If a Dockerfile specifies an `ENTRYPOINT`, Docker 1.12.x and earlier
does not allow you to unset it or specify an empty `--entrypoint` argument at
runtime.

### Running a container as a different user

If the Dockerfile does not specify a user, Docker containers run as the `root`
user (UID=0). If you are writing your own Dockerfile, you should specify a
non-root user if possible. If an image is created with additional users (such as
by adding the `useradd` to a `RUN` directive in the Dockerfile), you can specify
that user's login or UID using the `-u` or `--user` flag. The user must be valid
and have permission to run the command specified in the Dockerfile or at
runtime. However, you can specify a a numeric UID:GID pair to run a command as a
non-privileged user, even if the user or group does not explicitly exist on the
system.

### Overriding the image's working directory

Many images define a specific working directory or use the default working
directory for the user or UID specified by the image. If you do not specify a
working directory, the root directory (`/`) is used. To specify a different
working directory at runtime, use the `-w` or `--workdir` flag. If the working
directory does not exist, it will be created, but it will be created with the
owner `root`, and thus will not be accessible by non-privileged users.

### Capturing the container's ID on the host machine

You can use the `--cidfile="<path/to/file>"` flag to write the container's ID to
a file at runtime, in a similar way to how some UNIX processes write process ID
(PID) files. This may be useful for some automation workflows.

```bash
$ docker run --cidfile="/tmp/docker_test.cid" ubuntu "echo" "test"
```

If the file exists already, Docker will return an error.

### Mounting extra volumes

An image can be configured to use one or more named or anonymous volumes or
bind mounts as persistent storage, using the `--volume` or `-v` flag. Named
volumes, anonymous volumes, and bind mounts have the following differences:

- You can create a named volume using the `docker volume create` command, or
  when you start a container using the `-v volume_name:/container/mount/point`
  flag:
  ```bash
  $ docker volume create --name="myvol"
  ```
  ```bash
  $ docker run --rm -it -v myvol:/mnt/myvol ubuntu
  ```
  Named volumes persist even when no containers are using them.

- You can create anonymous volume on the fly using the `-v` flag in the
  `docker run` command, but with no name specified.
  ```bash
  $ docker run --rm -it -v /mnt/myvol ubuntu
  ```
  Anonymous volumes are removed automatically when the container that created
  them exits.

- You can mount a host filesystem into a container at runtime using the `-v`
  flag in the `docker run` command, with the host path as the first parameter
  and the mount point within the container as the second parameter.
  ```bash
  $ docker run --rm -it -v /tmp:/mnt/myvol ubuntu
  ```

Only anonymous volumes can be specified in a Dockerfile. See the
[Dockerfile reference](#volume) for more information.

You can specify multiple `--volume` flags to mount multiple volumes.

#### Duplicating another container's volumes

Use the `--volumes-from="<CONTAINER_NAME>"` flag to mount another container's
volumes, using the same mount points as the original container. You can specify
multiple `--volumes-from` flags and you can combine `--volumes-from` with
`--volume`. By default, volumes are mounted read-write, so containers that share
a volume can share files using the volume.

#### Viewing mounts from within a container

Volumes mounted within a container appear in the output of the `mount` command,
but may not all appear in the output of the `df` command, because they are likely
to share the same device, such as `/dev/vda2`. They may appear to be simple
directories within the root filesystem.

### Overriding or augmenting the image's environment variables

Docker automatically sets some environment variables, including `$HOME`,
`$HOSTNAME`, `$PATH`, and `$TERM`.

```bash
$ docker run --rm -it ubuntu "env"

PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
HOSTNAME=3f2bf37a85e5
TERM=xterm
no_proxy=*.local, 169.254/16
HOME=/root
```

In addition, you can specify extra environment variables
in the Dockerfile. You can override any of these environment variables or set
additional ones for a container at runtime. This example overrides `$HOME` and
sets a new environment variable `$TEST`.

```bash
$ docker run --rm -it --env="TEST=1" --env="HOME=/tmp" ubuntu "env"

PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
HOSTNAME=4985ff6127a6
TERM=xterm
TEST=1
HOME=/tmp
no_proxy=*.local, 169.254/16
```

You can use the `--env-file` flag to pass a file containing key-value pairs of
environment variables, one key per line. The following is an example of a
correctly-formatted environment variable file:

```bash
TEST="1"
PRODUCTION="0"
APP_NAME="my-app"
```

>**Warning**: Some images may not run correctly if expected environment
variables are missing or incorrectly set.

### Configuring your container's networking

Container networking is a big topic, and this topic will only discuss the
basics. For more thorough information about Docker networking, see
[Understand Docker container networks](https://docs.docker.com/engine/userguide/networking/).

#### Exposing and publishing extra ports

##### Exposing ports

You can use the `EXPOSE` Dockerfile directive to expose certain container ports
to other Docker processes and containers. **This does not make the port accessible
from the host machine.** You can expose ports not specified within the Dockerfile
using the `--expose` flag. To give the host machine access to exposed ports, you
need to publish them using the `-p` or `-P` directives described in the next
section.

The following example exposes port 80, in addition to any other ports the image
may expose by default. Remember that the host machine cannot access the port.

```bash
$ docker run --rm -i -t --expose "80" ubuntu "/bin/bash"
```

##### Publishing ports

Suppose you want to give the host machine access to the application the container
is running on port 80, but you know that the host is already using port 80. You
have a few options:

- Specify a port mapping for Docker to use. If you use syntax like `-p 8080:80`
  (to bind to `localhost`) or `-p 192.168.1.1:8080:80` (to bind to a specific
  host machine IP), Docker will map the container's port 80 to the host's
  port 8080. If port 8080 is already in use, an error is generated.

  ```bash
  $ docker run --rm -i -t --expose "80" -p "8080:80" ubuntu "/bin/bash"
  ```

- Let Docker map a single container port to a random host port. If you use syntax
  like `-p 80`, Docker will map the container's port 80 to a random host port. To
  see the port mappings, use the `docker port <container_name>` command.

  ```bash
  $ docker run --rm -i -t --expose "80" -p "80" ubuntu "/bin/bash"
  ```

- Let Docker map each exposed container port to a random host port, by using the
  `-P` flag. Again, to see the port mappings, use the
  `docker port <container_name>` command.

  ```bash
  docker run --rm -i -t --expose "80" -P ubuntu "/bin/bash"
  ```

##### Changing or disabling your container's network

By default, each container has a network that is unique within the Docker daemon
instance and not connected to the host machine or any other container. This
provides another level of isolation, for security and operations reasons. You can
direct a container to join the host's or another container's network namespace
by setting the `--network` flag to `host` or `"container:<container_name>"`.

To bridge with the host's network with the default Docker bridged network, set
`--network "bridge"`.

To disable your container's network entirely, set `--network none`.

Other networks may be available, including ones you have created within Docker.
To see all available networks for your container, use `docker network ls`.

##### Changing your container's hostname

By default, your container's hostname is the first part of your container's ID.
Unless your container is using `--network "host"`, you can override the hostname
using `--hostname "<hostname>".` For multiple containers using the same network,
the hostname must be unique.

##### Specifying your container's IP, IPv6, or MAC addresses

By default, your container uses a private IP and IPv6 address. To specify these
addresses manually, use `--ip="<IP-ADDRESS>"`and `--IPv6="<IPv6-ADDRESS>"`.

To specify a different MAC address for your container, use
`--mac-address="<MAC_ADDRESS>"`.

Be careful not to create any duplicate IP or MAC addresses within the same
network, because this can cause serious and difficult-to-diagnose networking
problems.

##### Setting your container's DNS settings

By default, the container's DNS servers and options (typically configured in
`/etc/resolv.conf`) are the same as those of the host, after removing references
to addresses which are local to the host machine. If the host's `/etc/resolv.conf`
file changes, stopped containers are updated automatically, and running containers
are updated when they are restarted.

To override your container's DNS servers, use `--dns="<IP-ADDRESS>"`
to specify the IP addresses of a nameserver. You can pass as many `--dns` flags
as you need.

To over ride your container's DNS search domains, use `--dns-search="<domain>"`.
YOu can pass as many `--dns-search` flags as you need. In a similar way, you can
override other `/etc/resolv.conf` settings using the `--dns-opt` flag. See your
operating system's documentation for `/etc/resolv.conf` for more details.

##### Adding a line to your container's hosts file

To add an entry to the container's `/etc/hosts` file, use the `--add-host` flag,
which takes a `hostname:IP-ADDRESS` pair.

### Customizing your container's logging behavior

By default Docker containers use the same log drivers as the Docker daemon. You
can specify the logging driver a container should use with the `--log-driver` flag.
The following logging drivers are supported, though the `docker log` command only
works with containers which use the `json-file` or `journald` driver. The default
logging driver is `json-file`. Configuration for the logging endpoints is out of
scope for this guide.

- **`none`**: This container will produce no logs.
- **`json-file`**: Writes log messages to a file using JSON format.
- **`syslog`**: Sends log messages to the `syslog` daemon, which handles them
                using its own configuration.
- **`journald`**: Writes log messages to `journald`, which handles them using its
                  own configuration.
- **`gelf`**: Writes log messages to a _Graylog Extended Log Format (GELF)_
              endpoint, such as GrayLog or LogStash.
- **`fluentd`**: Writes log messages to `fluentd`, which is an open source data
                 Collector for unified logging.
- **`awslogs`**: Writes log messages to Amazon CloudWatch Logs.
- **`splunk`**: Writes log messages to `splunk` using the Event HTTP Collector.


### Configuring your container's restart policy

A _restart policy_ determines whether or not to restart a container when it exits,
and if so, allows you to set some parameters to control the behavior of the actual
restart.

The default restart policy is `no`, which means that a container does not
start or restart when it exits or when the host machine restarts the Docker daemon
process.

When a container is restarted, attached clients are disconnected. This may cause
transient failures.

To view a container's current restart policy, use the
[`docker events`](commandline/events.md) command.

Use the `--restart="<RESTART_POLICY>"` flag to set a restart policy for a
container. The following policies are supported.

| Policy          | Result |
|-----------------|--------|
| `no`            | Do not automatically restart the container when it exits. This is the default. |
| `on-failure` or `on-failure:<MAX_RETRIES>` | Restart on failure, with an optional maximum number of retries before giving up. |
| `always`         | Always restart, trying an infinite number of times upon failure. Also start automatically when the Docker daemon starts. |
| `unless-stopped` | The same as `always`, but does not restart automatically if the container was explicitly stopped. |

In the case of `on-failure`, `always`, or `unless-stopped`, the restart is delayed
by an ever-doubling number of milliseconds, starting at 100, to prevent flooding
of the Docker daemon.

After a container starts and remains alive and reachable for at least 10 seconds,
the delay interval is reset to 100 ms. If a container is successfully restarted
(the container is started and runs for at least 10 seconds), the delay is reset
to its default value of 100 ms.

To show the number of attempted restarts for a given container or the last time
the container was restarted, use the
[`docker inspect`](commandline/inspect.md) command.

This example sets the restart policy on the `redis` container to `always`.

```bash
$ docker run --restart="always" redis
```

This example sets the restart policy on the `redis` container to `on-failure`
with 10 restart attempts before giving up.

```bash
$ docker run --restart="on-failure:10" redis
```

## Advanced options

These are some of the more advanced ways you can customize your containers at
runtime during the invocation of `docker run`.  Most of these options are used
less often and in special circumstances than the options previously discussed.

### Attaching local devices to your container

>**Warning**: If your container's functionality depends on devices on the host
machine, the container will not function correctly when run on systems which do
not have the same devices in the same configuration. This can make your containers
less portable.

To attach a local device, such as a USB drive, to your container at runtime, use
the `--device` flag, which takes a device path such as `/dev/cdrom`.

```bash
$ docker run --rm -it --device="/dev/usb0" ubuntu
```

>**Warning**: Attaching potentially-removable devices to a container may cause
the container to crash or behave in unexpected ways if the device disappears from
the host machine.

By default, the container has full access to the device. You can limit the
permissions by passing in one of `r` (read), `r` (write), or `m` (mknod) after
the device name, separated by a colon. This variant of the previous example makes
the USB drive read-only within the container.

```bash
$ docker run --rm -it --device="/dev/usb0:r" ubuntu
```

The device is present in the `/dev` filesystem as `/dev/usb0` but is not mounted.
For information on mounting devices, see the documentation for the `mount`
command in your distribution.

### Configuring your container's storage driver options

The default storage graph driver for your Docker daemon depends upon your distribution.
To determine the default storage driver, run `docker info`and look for output
such as the following:

```
Storage Driver: aufs
 Root Dir: /var/lib/docker/aufs
 Backing Filesystem: extfs
 Dirs: 246
 Dirperm1 Supported: true
```

If the storage driver is `devicemapper`, `btrfs`, `windowsfilter`, or `zfs`, you
can pass options to the storage driver using the `--storage-opt` flag. For
instance, the following command sets the size of the root filesystem for the
`fedora` container to 120 gigabytes if the underlying storage driver supports
it. If the driver does not support a given option, or the passing of options, an
error is generated.

```bash
$ docker run -it --storage-opt="size=120G" fedora "/bin/bash"
```

Each driver may support different options. Consult the documentation for that
storage driver to find out how you can configure it.

### Setting resource limits within your container using `ulimit`

To limit your container command's access to memory, number of user processes,
number of file locks, or other parameters which can be controlled by the `ulimit`
command, use the `--ulimit` flag. The argument to the flag takes the format
`limit_name:soft_limit:hard_limit`, and if the hard limit is omitted, the soft
limit becomes the hard limit.

>**Note**: The equivalent of `ulimit -a`, which reports all current limits, cannot
be used with the `--ulimit` flag.

The following example sets a soft limit of 50 processes, and a hard limit of 100.
It uses the `ulimit -u` command within the container to report the ulimit on
processes.

```bash
$ docker run --rm --ulimit nproc=50:100 ubuntu sh -c "ulimit -p"

50
```

### Constraining your container's resources at runtime

You can limit your container's hardware resources at runtime, including memory,
CPU resources, and block IO throughput. See
[Constrain a container's resources](https://docs.docker.com/engine/admin/resource_constraints/)
for full details.

### Configuring your container's security options

Docker provides a variety of different options configurable at runtime that fall
under the broad category of security. You can configure kernel namespaces, control
groups, kernel capabilities and more. This topic only covers a small subset of
these options. For more details, see
[Docker security](https://docs.docker.com/engine/security/security/).

#### Configuring or relaxing your container's namespace isolation

Namespace isolation is an advanced Docker topic that most users rarely need to
configure, and configuring it incorrectly can cause problems for other containers
or the host machine. The following are some of the ways that you can override a
container's different namespaces, which are used to isolate the container from
the host machine and other containers.

>**Warning**: Reducing the amount of isolation between a container and its host
or other containers is inherently insecure. Proceed with caution.

Namespaces in Docker fall into the a few broad groups:

* **`--pid`**: Set to `host` or `container:<CONTAINER_NAME>` to give this container
  kernel-level access to the host or container's processes, such as for debugging
  purposes.

  >**Warning**:If you use `--pid="host"`, your container may be able to shut down
  or restart the host machine.

* **`--uts`**: Set to `host` or `container:<CONTAINER_NAME>` to give this container
  access to manipulate the host or container's hostname or domain name.

* **`--ipc`**: Set to `host` or `container:<CONTAINER_NAME>` to give this container
  access to the host or container's IPC resources such as shared memory, semaphores,
  or message queues. For instance, you might use this for a database whose running
  processes are split among multiple containers.

For a given namespace, if you do not specify the flag, the container uses its
own namespace for that group. If you set the flag to `host`, the container shares
the host machine's namespace. If you set the flag to `container:<CONTAINER_NAME>`
the container shares the named container's namespace.

#### Configuring `selinux`, `apparmor`, and `seccomp` options

Docker provides a general-purpose `--security-opt` flag which you can use to pass
kernel-level options to the container at runtime. Discussion of `selinux`,
`apparmor`, and `seccomp` concepts is out of scope for this topic, but the
following examples will be helpful to those who already understand them.

>**Info**: Labels in `selinux` are a different concept from Docker labels.

```nohighlight
--security-opt="label=user:USER"     : Set the label user for the container
--security-opt="label=role:ROLE"     : Set the label role for the container
--security-opt="label=type:TYPE"     : Set the label type for the container
--security-opt="label=level:LEVEL"   : Set the label level for the container
--security-opt="label=disable"       : Turn off label confinement for the container
--security-opt="apparmor=PROFILE"    : Set the apparmor profile to be applied to the container
--security-opt="no-new-privileges"   : Disable container processes from gaining new privileges
--security-opt="seccomp=unconfined"  : Turn off seccomp confinement for the container
--security-opt="seccomp=profile.json": White listed syscalls seccomp Json file to be used as a seccomp filter
```

#### Running your container in privileged mode

By default, Docker containers run in _unprivileged_ mode. This limits a container's
ability to access any devices on the host machine or to do things such as run a
Docker daemon or container within a Docker container.

To run a container in privileged mode, and give it access to the host machine's
devices, use the `--privileged` flag. Running a container in privileged mode
removes much of the isolation between the container and the host machine, and thus
has security implications.

>**Warning**: If you intend to run a "Docker within Docker" setup,
as outlined in the
[Docker blog](https://blog.docker.com/2013/09/docker-can-now-run-within-docker/),
you may need to modify the `selinux`, AppArmor, or `lxc` configuration.

Be careful running a container in privileged mode. Normally, you cannot restart a
container by using the `shutdown` or `reboot` commands within it. If your container
is running in privileged mode, running one of these commands can potentially
stop or restart your host machine.

#### Configuring namespaced kernel parameters (sysctls) at runtime

Use the `--sysctl` to set namespaced kernel parameters (sysctls) in the
container. For example, the following example enables IP forwarding in the
container's network namespace:

```bash
$ docker run --sysctl net.ipv4.ip_forward=1 someimage
```


#### Currently supported sysctls

##### Caveats

- Not all sysctls are namespaced.
- Docker does not support changing sysctls inside a container if those sysctls
  can also modify the host system.
- The Linux kernel is expected to add namespacing to more sysctls in the future.

#### Configurable namespaced sysctls
  `IPC Namespace`:

  kernel.msgmax, kernel.msgmnb, kernel.msgmni, kernel.sem, kernel.shmall, kernel.shmmax, kernel.shmmni, kernel.shm_rmid_forced
  Sysctls beginning with fs.mqueue.*

  The `--ipc=host` option is not compatible with configuring these sysctls.

  `Network Namespace`:
      Sysctls beginning with net.*

  The `--network=host` option is not compatible with configuring these sysctls.

## Image settings you can't override

As the previous sections have shown, you can override and augment many of the
default settings for an image at runtime. However, there are a few image settings
that can only be set in the Dockerfile and cannot be overridden:

- **FROM**:       the image's base image.
- **MAINTAINER**: the image's maintainer, the person responsible for the image.
- **RUN**:        each `RUN` invocation in a Dockerfile represents a preparatory
                  command Docker runs when creating the image.
- **ADD**:        each `ADD` invocation adds files to the image, copying them
                  from the host where the image is built.

## Exit codes

If `docker run` exits with a non-zero status code, the exit code gives you some
information about what went wrong. Getting the exit status code depends upon
how you are running the command. If you use Bash or another variant of the `sh`
shell, the exit code is stored in the variable `$?`. Directly after running a command
that fails, issue `echo $?` to get the status code.

Docker follows the exit code standards set by the `chroot` command from FreeBSD:

| Exit code | Explanation |
|-----------|-------------|
| 125       | The error relates to the `docker` command itself. Often, this indicates a syntax error. |
| 126       | The command (the final argument to the `docker run` command) cannot be executed in the container, because it is not an executable file. |
| 127       | The command (the final argument to the `docker run` command)  cannot be found, perhaps because it is not installed or is not in the `$PATH` variable. |
| Any other exit code | Docker returns the exit code of the command within in the container. Refer to that command's documentation for more information. |

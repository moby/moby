---
title: "run"
description: "The run command description and usage"
keywords: "run, command, container"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# run

```markdown
Usage:  docker run [OPTIONS] IMAGE [COMMAND] [ARG...]

Run a command in a new container

Options:
      --add-host value              Add a custom host-to-IP mapping (host:ip) (default [])
  -a, --attach value                Attach to STDIN, STDOUT or STDERR (default [])
      --blkio-weight value          Block IO (relative weight), between 10 and 1000
      --blkio-weight-device value   Block IO weight (relative device weight) (default [])
      --cap-add value               Add Linux capabilities (default [])
      --cap-drop value              Drop Linux capabilities (default [])
      --cgroup-parent string        Optional parent cgroup for the container
      --cidfile string              Write the container ID to the file
      --cpu-count int               The number of CPUs available for execution by the container.
                                    Windows daemon only. On Windows Server containers, this is
                                    approximated as a percentage of total CPU usage.
      --cpu-percent int             Limit percentage of CPU available for execution
                                    by the container. Windows daemon only.
                                    The processor resource controls are mutually
                                    exclusive, the order of precedence is CPUCount
                                    first, then CPUShares, and CPUPercent last.
      --cpu-period int              Limit CPU CFS (Completely Fair Scheduler) period
      --cpu-quota int               Limit CPU CFS (Completely Fair Scheduler) quota
  -c, --cpu-shares int              CPU shares (relative weight)
      --cpus NanoCPUs               Number of CPUs (default 0.000)
      --cpu-rt-period int           Limit the CPU real-time period in microseconds
      --cpu-rt-runtime int          Limit the CPU real-time runtime in microseconds
      --cpuset-cpus string          CPUs in which to allow execution (0-3, 0,1)
      --cpuset-mems string          MEMs in which to allow execution (0-3, 0,1)
  -d, --detach                      Run container in background and print container ID
      --detach-keys string          Override the key sequence for detaching a container
      --device value                Add a host device to the container (default [])
      --device-cgroup-rule value    Add a rule to the cgroup allowed devices list
      --device-read-bps value       Limit read rate (bytes per second) from a device (default [])
      --device-read-iops value      Limit read rate (IO per second) from a device (default [])
      --device-write-bps value      Limit write rate (bytes per second) to a device (default [])
      --device-write-iops value     Limit write rate (IO per second) to a device (default [])
      --disable-content-trust       Skip image verification (default true)
      --dns value                   Set custom DNS servers (default [])
      --dns-option value            Set DNS options (default [])
      --dns-search value            Set custom DNS search domains (default [])
      --entrypoint string           Overwrite the default ENTRYPOINT of the image
  -e, --env value                   Set environment variables (default [])
      --env-file value              Read in a file of environment variables (default [])
      --expose value                Expose a port or a range of ports (default [])
      --group-add value             Add additional groups to join (default [])
      --health-cmd string           Command to run to check health
      --health-interval duration    Time between running the check (ns|us|ms|s|m|h) (default 0s)
      --health-retries int          Consecutive failures needed to report unhealthy
      --health-timeout duration     Maximum time to allow one check to run (ns|us|ms|s|m|h) (default 0s)
      --help                        Print usage
  -h, --hostname string             Container host name
      --init                        Run an init inside the container that forwards signals and reaps processes
      --init-path string            Path to the docker-init binary
  -i, --interactive                 Keep STDIN open even if not attached
      --io-maxbandwidth string      Maximum IO bandwidth limit for the system drive (Windows only)
                                    (Windows only). The format is `<number><unit>`.
                                    Unit is optional and can be `b` (bytes per second),
                                    `k` (kilobytes per second), `m` (megabytes per second),
                                    or `g` (gigabytes per second). If you omit the unit,
                                    the system uses bytes per second.
                                    --io-maxbandwidth and --io-maxiops are mutually exclusive options.
      --io-maxiops uint             Maximum IOps limit for the system drive (Windows only)
      --ip string                   IPv4 address (e.g., 172.30.100.104)
      --ip6 string                  IPv6 address (e.g., 2001:db8::33)
      --ipc string                  IPC namespace to use
      --isolation string            Container isolation technology
      --kernel-memory string        Kernel memory limit
  -l, --label value                 Set meta data on a container (default [])
      --label-file value            Read in a line delimited file of labels (default [])
      --link value                  Add link to another container (default [])
      --link-local-ip value         Container IPv4/IPv6 link-local addresses (default [])
      --log-driver string           Logging driver for the container
      --log-opt value               Log driver options (default [])
      --mac-address string          Container MAC address (e.g., 92:d0:c6:0a:29:33)
  -m, --memory string               Memory limit
      --memory-reservation string   Memory soft limit
      --memory-swap string          Swap limit equal to memory plus swap: '-1' to enable unlimited swap
      --memory-swappiness int       Tune container memory swappiness (0 to 100) (default -1)
      --name string                 Assign a name to the container
      --network-alias value         Add network-scoped alias for the container (default [])
      --network string              Connect a container to a network
                                    'bridge': create a network stack on the default Docker bridge
                                    'none': no networking
                                    'container:<name|id>': reuse another container's network stack
                                    'host': use the Docker host network stack
                                    '<network-name>|<network-id>': connect to a user-defined network
      --no-healthcheck              Disable any container-specified HEALTHCHECK
      --oom-kill-disable            Disable OOM Killer
      --oom-score-adj int           Tune host's OOM preferences (-1000 to 1000)
      --pid string                  PID namespace to use
      --pids-limit int              Tune container pids limit (set -1 for unlimited)
      --privileged                  Give extended privileges to this container
  -p, --publish value               Publish a container's port(s) to the host (default [])
  -P, --publish-all                 Publish all exposed ports to random ports
      --read-only                   Mount the container's root filesystem as read only
      --restart string              Restart policy to apply when a container exits (default "no")
                                    Possible values are : no, on-failure[:max-retry], always, unless-stopped
      --rm                          Automatically remove the container when it exits
      --runtime string              Runtime to use for this container
      --security-opt value          Security Options (default [])
      --shm-size bytes              Size of /dev/shm
                                    The format is `<number><unit>`. `number` must be greater than `0`.
                                    Unit is optional and can be `b` (bytes), `k` (kilobytes), `m` (megabytes),
                                    or `g` (gigabytes). If you omit the unit, the system uses bytes.
      --sig-proxy                   Proxy received signals to the process (default true)
      --stop-signal string          Signal to stop a container, SIGTERM by default (default "SIGTERM")
      --stop-timeout=10             Timeout (in seconds) to stop a container
      --storage-opt value           Storage driver options for the container (default [])
      --sysctl value                Sysctl options (default map[])
      --tmpfs value                 Mount a tmpfs directory (default [])
  -t, --tty                         Allocate a pseudo-TTY
      --ulimit value                Ulimit options (default [])
  -u, --user string                 Username or UID (format: <name|uid>[:<group|gid>])
      --userns string               User namespace to use
                                    'host': Use the Docker host user namespace
                                    '': Use the Docker daemon user namespace specified by `--userns-remap` option.
      --uts string                  UTS namespace to use
  -v, --volume value                Bind mount a volume (default []). The format
                                    is `[host-src:]container-dest[:<options>]`.
                                    The comma-delimited `options` are [rw|ro],
                                    [z|Z], [[r]shared|[r]slave|[r]private], and
                                    [nocopy]. The 'host-src' is an absolute path
                                    or a name value.
      --volume-driver string        Optional volume driver for the container
      --volumes-from value          Mount volumes from the specified container(s) (default [])
  -w, --workdir string              Working directory inside the container
```

The `docker run` command first `creates` a writeable container layer over the
specified image, and then `starts` it using the specified command. That is,
`docker run` is equivalent to the API `/containers/create` then
`/containers/(id)/start`. A stopped container can be restarted with all its
previous changes intact using `docker start`. See `docker ps -a` to view a list
of all containers.

The `docker run` command can be used in combination with `docker commit` to
[*change the command that a container runs*](commit.md).

See the [Docker run reference](../run.md) for more details and examples about
what you can do with the `docker run` command. A few examples are listed below.

For information on connecting a container to a network, see the ["*Docker network overview*"](https://docs.docker.com/engine/userguide/networking/).

## Examples

The `docker run` command is powerful and has many options which can augment or
override your container's Dockerfile settings at runtime. See the
[Docker run reference](../run.md) for more thorough examples using the
`docker run` command. The following are only a few examples.

### Simple examples

Run the `hello-world` container to test your Docker installation.

```bash
$ docker run hello-world
```

Run the `sh` command in an `alpine` container in interactive mode. Remove the
container when it stops. To stop the container, type `exit`.

```bash
docker run --rm -it alpine sh
```

Run a `nginx` container as a daemon process (in the background), mapping port 80
to port 8080 on the host machine. To access the web server, go to
http://<host_ip>:8080/.

```bash
$ docker run -d -p 8080:80 nginx
```

### More complex examples

#### Granting kernel-level capabilities

By default, a container does not have the ability to manipulate the host
machine. If your container needs the ability to mount or unmount disks, start
or stop network interfaces, run a debugger against a host process, or other
privileged operations, you can grant kernel-level capabilities to the container,
using the `--cap-add` flag. For a full list of capabilities, view the
`capabilities` man page by running the command `man 7 capabilities` on your host.

Granting capabilities reduces the isolation of your container and represents a
security compromise. For this reason, always grant the minimum capability that
will allow the container to accomplish the task.

The following example shows a failed attempt by an unprivileged container to
mount a filesystem.

```bash
$ docker run --rm --it ubuntu bash
root@bc338942ef20:/# mount -t tmpfs none /mnt
mount: permission denied
```

In order to mount filesystems, the container needs the kernel capability
`cap_sys_admin`. The following container is given that capability, and the
`mount` command succeeds.

```bash
$ docker run -it --cap-add="SYS_ADMIN" ubuntu bash
root@50e3f57e16e6:/# mount -t tmpfs none /mnt
root@50e3f57e16e6:/# df -h
Filesystem      Size  Used Avail Use% Mounted on
none            1.9G     0  1.9G   0% /mnt
```

**Note**: Docker automatically prepends the `CAP_` to the beginning of the
capability. In this case, the capability is called `CAP_SYS_ADMIN`, so you
pass `--cap-add="SYS_ADMIN"`, which Docker transforms into `CAP_SYS_ADMIN`.
By convention, capital letters are used for capability names.

To grant all capabilities, you can use the `--privileged` flag. However, this is
not recommended for most situations because it is potentially dangerous to the
host machine and other containers running on that host. One case which requires
the `--privileged` flag is running the Docker daemon inside a container.

#### Mount tmpfs (--tmpfs)

    $ docker run -d --tmpfs /run:rw,noexec,nosuid,size=65536k my_image

The `--tmpfs` flag mounts an empty tmpfs into the container with the `rw`,
`noexec`, `nosuid`, `size=65536k` options.

#### Mount Docker volumes or host filesystems into a container

To mount a volume managed by Docker or a host filesystem (bind mount) into a
container at runtime, use the `-v` or `--volume` flag. By default, the volume or
filesystem is mounted read-write, so that the container can both read from and
write to the filesystem. To limit the container's access to read-only, use the
`--read-only` flag.

Named volumes are managed by Docker, and a named volume's lifecycle is independent
of the lifecycles of containers using it. The first example mounts a volume
named `myvol` into the container at mount point `/myvol/`. If the volume does
not yet exist, it is created automatically, and it persists even if no running
container is using it.

```bash
$ docker run -v myvol:/myvol -w /myvol -i -t busybox bash
```

Anonymous volumes are also managed by Docker, but an anonymous volume's name is
randomly generated and an anonymous volume is generally only used by the
container that created it. If a second container uses an anonymous volume, it
is not removed automatically, but behaves like a named volume. To create an
anonymous volume, do not specify a name. This example is the same as the
previous one, but uses an anonymous volume.

```bash
$ docker run -v /myvol -w /myvol -i -t busybox bash
```

This example uses a bind mount to a host directory which does not yet exist.
Docker creates the host volume automatically before starting the container. If
the host or another container writes data into the directory, this container can
read it.

```bash
$ docker run -v /doesnt/exist:/foo -w /foo -i -t busybox bash
```

This example mounts the same directory, but mounts the volume read-only by
appending `ro:` to the mount point. The `touch` command fails.

```bash
$ docker run -v /doesnt/exist:/foo:ro -w /foo -i -t busybox touch /foo/testfile

touch: cannot touch '/foo/testfile': Read-only file system
```

On Windows, the paths must be specified using Windows-style semantics.


    PS C:\> docker run -v c:\foo:c:\dest microsoft/nanoserver cmd /s /c type c:\dest\somefile.txt
    Contents of file

    PS C:\> docker run -v c:\foo:d: microsoft/nanoserver cmd /s /c type d:\somefile.txt
    Contents of file

The following examples will fail when using Windows-based containers, as the
destination of a volume or bind-mount inside the container must be one of:
a non-existing or empty directory; or a drive other than C:. Further, the source
of a bind mount must be a local directory, not a file.

    net use z: \\remotemachine\share
    docker run -v z:\foo:c:\dest ...
    docker run -v \\uncpath\to\directory:c:\dest ...
    docker run -v c:\foo\somefile.txt:c:\dest ...
    docker run -v c:\foo:c: ...
    docker run -v c:\foo:c:\existing-directory-with-contents ...

For in-depth information about volumes, refer to [manage data in containers](https://docs.docker.com/engine/tutorials/dockervolumes/)


Volumes can be used in combination with `--read-only` to control where
a container can write files. This example mounts an anonymous volume mounted
on `/icanwrite/` in the container. Because of the `--read-only` flag, the
container's root filesystem is read-only. The `touch` command succeeds because
the `/icanwrite` filesystem is still read-write.

```bash
$ docker run --read-only -v /icanwrite busybox touch /icanwrite/testfile
```

This command uses a feature of Bash and and other similar shells called _command
substitution_ to run the `pwd` command and use its output in several different
places. The `pwd` command outputs the directory on the host where the current
command is being run.

```bash
$ docker  run  -v `pwd`:`pwd` -w `pwd` -i -t  ubuntu pwd
```

The `-v` flag mounts the current working directory into the container at the
same path. The `-w` sets the container's working directory to the mounted
directory. This combination of flags might be helpful in running a script or
command which must be run within a specific directory structure.

This example provides you with the ability to create and manipulate the host
machine's Docker daemon, by mounting the running Docker's Unix socket and running
a statically linked Docker binary. This has security implications, so use this
method at your own risk. To get the statically linked binary, refer to
[get the linux binary](
../../installation/binaries.md#get-the-linux-binary)).

```bash
$ docker run -t -i -v /var/run/docker.sock:/var/run/docker.sock -v /path/to/static-docker-binary:/usr/bin/docker busybox sh
```

For in-depth information about volumes, refer to [manage data in containers](https://docs.docker.com/engine/tutorials/dockervolumes/)

#### Publish or expose port (-p, --expose)

By default, all ports on a given container are accessible by other **containers**
on the same network (except the default `bridge` network), but are not accessible
from external hosts, because the host machine does not route to or from them. If
you want to make a port on a container accessible to external hosts, you can
_publish_ them. You do not need to publish a port if it will only be used by
other containers. For instance, if you run a `mysql` container which is only
used by a `wordpress` container on the same network, you do not need to publish
the `mysql` port. However, if you want web browsers on external hosts to be able
to connect to the `wordpress` container, you do need to publish its HTTP port.

The format for the `-p` or `--publish` flag is
`HOST_IP:HOST_PORT:CONTAINER_IP:CONTAINER_PORT`. The `HOST_IP` and `CONTAINER_IP`
are optional, and if you omit them, the IP `0.0.0.0` is used, which binds all
IP addresses. If you leave off `HOST_PORT`, the container port is bound to a
random port higher than port 3000.

The following example maps port 80 on the container to port 8080 on the host
machine. This means that if someone browses `http://<HOST_IP>:8080`, the
connection will be routed to `http://<CONTAINER_IP>:80`.

```bash
$ docker run -p 8080:80 ubuntu bash
```

In addition to publishing a port, you can _expose_ a port. Exposing a port adds
metadata to the information Docker has about the container by adding an
`ExposedPorts` section to the container's configuration, which you can see in
the output of `docker inspect`. In addition, if you have exposed ports for a
container and you use the `-P` flag at runtime, which publishes all exposed
ports), then each exposed port is automatically published to a random port
numbered higher than 3000.

```bash
$ docker run --expose 80 ubuntu bash
```

This adds metadata to the container's `docker inspect` output that port 80
is exposed.

```bash
$ docker run --expose 80 -P ubuntu bash
```

This adds metadata to the container's configuration indicating that port 80
is exposed, and maps port 80 to a random port numbered higher than 3000 on the
host machine.

The
[Docker User Guide](https://docs.docker.com/engine/userguide/networking/default_network/dockerlinks/)
provides more information about how and when to publish and expose container
ports.


#### Set metadata on container (-l, --label, --label-file)

For information on working with labels, see [*Labels - custom
metadata in Docker*](../../userguide/labels-custom-metadata.md) in the Docker
User Guide.

#### Connect a container to a network (--network)

To connect a container to a network managed by Docker, use the `--network` flag.
This example adds the `busybox` container to the `my-net` network.

```bash
$ docker run -itd --network="my-net" busybox
```

See the [Docker run reference](../run.md) for more details about configuring
network settings at runtime.


#### Restart policies (--restart)

See the [Docker run reference](../run.md) for details and examples about setting
restart policies on containers.

The `--stop-timeout` flag sets the timeout (in seconds) that a pre-defined (see `--stop-signal`) system call
signal that will be sent to the container to exit. After timeout elapses the container will be killed with SIGKILL.

#### Stop container with signal (--stop-signal)

The `--stop-signal` flag sets the system call signal that will be sent to the container to exit.
This signal can be a valid unsigned number that matches a position in the kernel's syscall table, for instance 9,
or a signal name in the format SIGNAME, for instance SIGKILL.

#### Specify isolation technology for container (--isolation)

This option is useful in situations where you are running Docker containers on
Windows. The `--isolation <value>` option sets a container's isolation technology.
On Linux, the only supported is the `default` option which uses
Linux namespaces. These two commands are equivalent on Linux:

```bash
$ docker run -d busybox top
$ docker run -d --isolation default busybox top
```

On Windows, `--isolation` can take one of these values:


| Value     | Description                                                                                |
|-----------|--------------------------------------------------------------------------------------------|
| `default` | Use the value specified by the Docker daemon's `--exec-opt` or system default (see below). |
| `process` | Shared-kernel namespace isolation (not supported on Windows client operating systems).     |
| `hyperv`  | Hyper-V hypervisor partition-based isolation.                                              |

The default isolation on Windows server operating systems is `process`. The default (and only supported)
isolation on Windows client operating systems is `hyperv`. An attempt to start a container on a client
operating system with `--isolation process` will fail.

On Windows server, assuming the default configuration, these commands are equivalent
and result in `process` isolation:

```PowerShell
PS C:\> docker run -d microsoft/nanoserver powershell echo process
PS C:\> docker run -d --isolation default microsoft/nanoserver powershell echo process
PS C:\> docker run -d --isolation process microsoft/nanoserver powershell echo process
```

If you have set the `--exec-opt isolation=hyperv` option on the Docker `daemon`, or
are running against a Windows client-based daemon, these commands are equivalent and
result in `hyperv` isolation:

```PowerShell
PS C:\> docker run -d microsoft/nanoserver powershell echo hyperv
PS C:\> docker run -d --isolation default microsoft/nanoserver powershell echo hyperv
PS C:\> docker run -d --isolation hyperv microsoft/nanoserver powershell echo hyperv
```

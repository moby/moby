<!--[metadata]>
+++
title = "run"
description = "The run command description and usage"
keywords = ["run, command, container"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# run

    Usage: docker run [OPTIONS] IMAGE [COMMAND] [ARG...]

    Run a command in a new container

      -a, --attach=[]               Attach to STDIN, STDOUT or STDERR
      --add-host=[]                 Add a custom host-to-IP mapping (host:ip)
      --blkio-weight=0              Block IO weight (relative weight)
      --blkio-weight-device=[]      Block IO weight (relative device weight, format: `DEVICE_NAME:WEIGHT`)
      --cpu-shares=0                CPU shares (relative weight)
      --cap-add=[]                  Add Linux capabilities
      --cap-drop=[]                 Drop Linux capabilities
      --cgroup-parent=""            Optional parent cgroup for the container
      --cidfile=""                  Write the container ID to the file
      --cpu-percent=0               Limit percentage of CPU available for execution by the container. Windows daemon only.
      --cpu-period=0                Limit CPU CFS (Completely Fair Scheduler) period
      --cpu-quota=0                 Limit CPU CFS (Completely Fair Scheduler) quota
      --cpuset-cpus=""              CPUs in which to allow execution (0-3, 0,1)
      --cpuset-mems=""              Memory nodes (MEMs) in which to allow execution (0-3, 0,1)
      -d, --detach                  Run container in background and print container ID
      --detach-keys                 Specify the escape key sequence used to detach a container
      --device=[]                   Add a host device to the container
      --device-read-bps=[]          Limit read rate (bytes per second) from a device (e.g., --device-read-bps=/dev/sda:1mb)
      --device-read-iops=[]         Limit read rate (IO per second) from a device (e.g., --device-read-iops=/dev/sda:1000)
      --device-write-bps=[]         Limit write rate (bytes per second) to a device (e.g., --device-write-bps=/dev/sda:1mb)
      --device-write-iops=[]        Limit write rate (IO per second) to a device (e.g., --device-write-bps=/dev/sda:1000)
      --disable-content-trust=true  Skip image verification
      --dns=[]                      Set custom DNS servers
      --dns-opt=[]                  Set custom DNS options
      --dns-search=[]               Set custom DNS search domains
      -e, --env=[]                  Set environment variables
      --entrypoint=""               Overwrite the default ENTRYPOINT of the image
      --env-file=[]                 Read in a file of environment variables
      --expose=[]                   Expose a port or a range of ports
      --group-add=[]                Add additional groups to run as
      -h, --hostname=""             Container host name
      --help                        Print usage
      -i, --interactive             Keep STDIN open even if not attached
      --ip=""                       Container IPv4 address (e.g. 172.30.100.104)
      --ip6=""                      Container IPv6 address (e.g. 2001:db8::33)
      --ipc=""                      IPC namespace to use
      --isolation=""                Container isolation technology
      --kernel-memory=""            Kernel memory limit
      -l, --label=[]                Set metadata on the container (e.g., --label=com.example.key=value)
      --label-file=[]               Read in a file of labels (EOL delimited)
      --link=[]                     Add link to another container
      --log-driver=""               Logging driver for container
      --log-opt=[]                  Log driver specific options
      -m, --memory=""               Memory limit
      --mac-address=""              Container MAC address (e.g. 92:d0:c6:0a:29:33)
      --io-maxbandwidth=""          Maximum IO bandwidth limit for the system drive
                                    (Windows only). The format is `<number><unit>`.
                                    Unit is optional and can be `b` (bytes per second),
                                    `k` (kilobytes per second), `m` (megabytes per second),
                                    or `g` (gigabytes per second). If you omit the unit,
                                    the system uses bytes per second.
                                    --io-maxbandwidth and --io-maxiops are mutually exclusive options.
      --io-maxiops=0                Maximum IO per second limit for the system drive (Windows only).
                                    --io-maxbandwidth and --io-maxiops are mutually exclusive options.
      --memory-reservation=""       Memory soft limit
      --memory-swap=""              A positive integer equal to memory plus swap. Specify -1 to enable unlimited swap.
      --memory-swappiness=""        Tune a container's memory swappiness behavior. Accepts an integer between 0 and 100.
      --name=""                     Assign a name to the container
      --net="bridge"                Connect a container to a network
                                    'bridge': create a network stack on the default Docker bridge
                                    'none': no networking
                                    'container:<name|id>': reuse another container's network stack
                                    'host': use the Docker host network stack
                                    '<network-name>|<network-id>': connect to a user-defined network
      --net-alias=[]                Add network-scoped alias for the container
      --oom-kill-disable            Whether to disable OOM Killer for the container or not
      --oom-score-adj=0             Tune the host's OOM preferences for containers (accepts -1000 to 1000)
      -P, --publish-all             Publish all exposed ports to random ports
      -p, --publish=[]              Publish a container's port(s) to the host
      --pid=""                      PID namespace to use
      --pids-limit=-1                Tune container pids limit (set -1 for unlimited), kernel >= 4.3
      --privileged                  Give extended privileges to this container
      --read-only                   Mount the container's root filesystem as read only
      --restart="no"                Restart policy (no, on-failure[:max-retry], always, unless-stopped)
      --rm                          Automatically remove the container when it exits
      --shm-size=[]                 Size of `/dev/shm`. The format is `<number><unit>`. `number` must be greater than `0`.  Unit is optional and can be `b` (bytes), `k` (kilobytes), `m` (megabytes), or `g` (gigabytes). If you omit the unit, the system uses bytes. If you omit the size entirely, the system uses `64m`.
      --security-opt=[]             Security Options
      --sig-proxy=true              Proxy received signals to the process
      --stop-signal="SIGTERM"       Signal to stop a container
      --storage-opt=[]              Set storage driver options per container
      --sysctl[=*[]*]]              Configure namespaced kernel parameters at runtime
      -t, --tty                     Allocate a pseudo-TTY
      -u, --user=""                 Username or UID (format: <name|uid>[:<group|gid>])
      --userns=""                   Container user namespace
                                    'host': Use the Docker host user namespace
                                    '': Use the Docker daemon user namespace specified by `--userns-remap` option.
      --ulimit=[]                   Ulimit options
      --uts=""                      UTS namespace to use
      -v, --volume=[host-src:]container-dest[:<options>]
                                    Bind mount a volume. The comma-delimited
                                    `options` are [rw|ro], [z|Z],
                                    [[r]shared|[r]slave|[r]private], and
                                    [nocopy]. The 'host-src' is an absolute path
                                    or a name value.
      --volume-driver=""            Container's volume driver
      --volumes-from=[]             Mount volumes from the specified container(s)
      -w, --workdir=""              Working directory inside the container

The `docker run` command first `creates` a writeable container layer over the
specified image, and then `starts` it using the specified command. That is,
`docker run` is equivalent to the API `/containers/create` then
`/containers/(id)/start`. A stopped container can be restarted with all its
previous changes intact using `docker start`. See `docker ps -a` to view a list
of all containers.

The `docker run` command can be used in combination with `docker commit` to
[*change the command that a container runs*](commit.md). There is additional detailed information about `docker run` in the [Docker run reference](../run.md).

For information on connecting a container to a network, see the ["*Docker network overview*"](../../userguide/networking/index.md).

## Examples

### Assign name and allocate pseudo-TTY (--name, -it)

    $ docker run --name test -it debian
    root@d6c0fe130dba:/# exit 13
    $ echo $?
    13
    $ docker ps -a | grep test
    d6c0fe130dba        debian:7            "/bin/bash"         26 seconds ago      Exited (13) 17 seconds ago                         test

This example runs a container named `test` using the `debian:latest`
image. The `-it` instructs Docker to allocate a pseudo-TTY connected to
the container's stdin; creating an interactive `bash` shell in the container.
In the example, the `bash` shell is quit by entering
`exit 13`. This exit code is passed on to the caller of
`docker run`, and is recorded in the `test` container's metadata.

### Capture container ID (--cidfile)

    $ docker run --cidfile /tmp/docker_test.cid ubuntu echo "test"

This will create a container and print `test` to the console. The `cidfile`
flag makes Docker attempt to create a new file and write the container ID to it.
If the file exists already, Docker will return an error. Docker will close this
file when `docker run` exits.

### Full container capabilities (--privileged)

    $ docker run -t -i --rm ubuntu bash
    root@bc338942ef20:/# mount -t tmpfs none /mnt
    mount: permission denied

This will *not* work, because by default, most potentially dangerous kernel
capabilities are dropped; including `cap_sys_admin` (which is required to mount
filesystems). However, the `--privileged` flag will allow it to run:

    $ docker run -t -i --privileged ubuntu bash
    root@50e3f57e16e6:/# mount -t tmpfs none /mnt
    root@50e3f57e16e6:/# df -h
    Filesystem      Size  Used Avail Use% Mounted on
    none            1.9G     0  1.9G   0% /mnt

The `--privileged` flag gives *all* capabilities to the container, and it also
lifts all the limitations enforced by the `device` cgroup controller. In other
words, the container can then do almost everything that the host can do. This
flag exists to allow special use-cases, like running Docker within Docker.

### Set working directory (-w)

    $ docker  run -w /path/to/dir/ -i -t  ubuntu pwd

The `-w` lets the command being executed inside directory given, here
`/path/to/dir/`. If the path does not exists it is created inside the container.

### Set storage driver options per container

    $ docker create -it --storage-opt size=120G fedora /bin/bash

This (size) will allow to set the container rootfs size to 120G at creation time. 
User cannot pass a size less than the Default BaseFS Size.

### Mount tmpfs (--tmpfs)

    $ docker run -d --tmpfs /run:rw,noexec,nosuid,size=65536k my_image

The `--tmpfs` flag mounts an empty tmpfs into the container with the `rw`,
`noexec`, `nosuid`, `size=65536k` options.

### Mount volume (-v, --read-only)

    $ docker  run  -v `pwd`:`pwd` -w `pwd` -i -t  ubuntu pwd

The `-v` flag mounts the current working directory into the container. The `-w`
lets the command being executed inside the current working directory, by
changing into the directory to the value returned by `pwd`. So this
combination executes the command using the container, but inside the
current working directory.

    $ docker run -v /doesnt/exist:/foo -w /foo -i -t ubuntu bash

When the host directory of a bind-mounted volume doesn't exist, Docker
will automatically create this directory on the host for you. In the
example above, Docker will create the `/doesnt/exist`
folder before starting your container.

    $ docker run --read-only -v /icanwrite busybox touch /icanwrite here

Volumes can be used in combination with `--read-only` to control where
a container writes files. The `--read-only` flag mounts the container's root
filesystem as read only prohibiting writes to locations other than the
specified volumes for the container.

    $ docker run -t -i -v /var/run/docker.sock:/var/run/docker.sock -v /path/to/static-docker-binary:/usr/bin/docker busybox sh

By bind-mounting the docker unix socket and statically linked docker
binary (refer to [get the linux binary](
../../installation/binaries.md#get-the-linux-binary)),
you give the container the full access to create and manipulate the host's
Docker daemon.

### Publish or expose port (-p, --expose)

    $ docker run -p 127.0.0.1:80:8080 ubuntu bash

This binds port `8080` of the container to port `80` on `127.0.0.1` of the host
machine. The [Docker User
Guide](../../userguide/networking/default_network/dockerlinks.md)
explains in detail how to manipulate ports in Docker.

    $ docker run --expose 80 ubuntu bash

This exposes port `80` of the container without publishing the port to the host
system's interfaces.

### Set environment variables (-e, --env, --env-file)

    $ docker run -e MYVAR1 --env MYVAR2=foo --env-file ./env.list ubuntu bash

This sets simple (non-array) environmental variables in the container. For
illustration all three
flags are shown here. Where `-e`, `--env` take an environment variable and
value, or if no `=` is provided, then that variable's current value, set via
`export`, is passed through (i.e. `$MYVAR1` from the host is set to `$MYVAR1`
in the container). When no `=` is provided and that variable is not defined
in the client's environment then that variable will be removed from the
container's list of environment variables. All three flags, `-e`, `--env` and
`--env-file` can be repeated.

Regardless of the order of these three flags, the `--env-file` are processed
first, and then `-e`, `--env` flags. This way, the `-e` or `--env` will
override variables as needed.

    $ cat ./env.list
    TEST_FOO=BAR
    $ docker run --env TEST_FOO="This is a test" --env-file ./env.list busybox env | grep TEST_FOO
    TEST_FOO=This is a test

The `--env-file` flag takes a filename as an argument and expects each line
to be in the `VAR=VAL` format, mimicking the argument passed to `--env`. Comment
lines need only be prefixed with `#`

An example of a file passed with `--env-file`

    $ cat ./env.list
    TEST_FOO=BAR

    # this is a comment
    TEST_APP_DEST_HOST=10.10.0.127
    TEST_APP_DEST_PORT=8888
    _TEST_BAR=FOO
    TEST_APP_42=magic
    helloWorld=true
    123qwe=bar
    org.spring.config=something

    # pass through this variable from the caller
    TEST_PASSTHROUGH
    $ TEST_PASSTHROUGH=howdy docker run --env-file ./env.list busybox env
    PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
    HOSTNAME=5198e0745561
    TEST_FOO=BAR
    TEST_APP_DEST_HOST=10.10.0.127
    TEST_APP_DEST_PORT=8888
    _TEST_BAR=FOO
    TEST_APP_42=magic
    helloWorld=true
    TEST_PASSTHROUGH=howdy
    HOME=/root
    123qwe=bar
    org.spring.config=something

    $ docker run --env-file ./env.list busybox env
    PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
    HOSTNAME=5198e0745561
    TEST_FOO=BAR
    TEST_APP_DEST_HOST=10.10.0.127
    TEST_APP_DEST_PORT=8888
    _TEST_BAR=FOO
    TEST_APP_42=magic
    helloWorld=true
    TEST_PASSTHROUGH=
    HOME=/root
    123qwe=bar
    org.spring.config=something

### Set metadata on container (-l, --label, --label-file)

A label is a `key=value` pair that applies metadata to a container. To label a container with two labels:

    $ docker run -l my-label --label com.example.foo=bar ubuntu bash

The `my-label` key doesn't specify a value so the label defaults to an empty
string(`""`). To add multiple labels, repeat the label flag (`-l` or `--label`).

The `key=value` must be unique to avoid overwriting the label value. If you
specify labels with identical keys but different values, each subsequent value
overwrites the previous. Docker uses the last `key=value` you supply.

Use the `--label-file` flag to load multiple labels from a file. Delimit each
label in the file with an EOL mark. The example below loads labels from a
labels file in the current directory:

    $ docker run --label-file ./labels ubuntu bash

The label-file format is similar to the format for loading environment
variables. (Unlike environment variables, labels are not visible to processes
running inside a container.) The following example illustrates a label-file
format:

    com.example.label1="a label"

    # this is a comment
    com.example.label2=another\ label
    com.example.label3

You can load multiple label-files by supplying multiple  `--label-file` flags.

For additional information on working with labels, see [*Labels - custom
metadata in Docker*](../../userguide/labels-custom-metadata.md) in the Docker User
Guide.

### Connect a container to a network (--net)

When you start a container use the `--net` flag to connect it to a network.
This adds the `busybox` container to the `my-net` network.

```bash
$ docker run -itd --net=my-net busybox
```

You can also choose the IP addresses for the container with `--ip` and `--ip6`
flags when you start the container on a user-defined network.

```bash
$ docker run -itd --net=my-net --ip=10.10.9.75 busybox
```

If you want to add a running container to a network use the `docker network connect` subcommand.

You can connect multiple containers to the same network. Once connected, the
containers can communicate easily need only another container's IP address
or name. For `overlay` networks or custom plugins that support multi-host
connectivity, containers connected to the same multi-host network but launched
from different Engines can also communicate in this way.

**Note**: Service discovery is unavailable on the default bridge network.
Containers can communicate via their IP addresses by default. To communicate
by name, they must be linked.

You can disconnect a container from a network using the `docker network
disconnect` command.

### Mount volumes from container (--volumes-from)

    $ docker run --volumes-from 777f7dc92da7 --volumes-from ba8c0c54f0f2:ro -i -t ubuntu pwd

The `--volumes-from` flag mounts all the defined volumes from the referenced
containers. Containers can be specified by repetitions of the `--volumes-from`
argument. The container ID may be optionally suffixed with `:ro` or `:rw` to
mount the volumes in read-only or read-write mode, respectively. By default,
the volumes are mounted in the same mode (read write or read only) as
the reference container.

Labeling systems like SELinux require that proper labels are placed on volume
content mounted into a container. Without a label, the security system might
prevent the processes running inside the container from using the content. By
default, Docker does not change the labels set by the OS.

To change the label in the container context, you can add either of two suffixes
`:z` or `:Z` to the volume mount. These suffixes tell Docker to relabel file
objects on the shared volumes. The `z` option tells Docker that two containers
share the volume content. As a result, Docker labels the content with a shared
content label. Shared volume labels allow all containers to read/write content.
The `Z` option tells Docker to label the content with a private unshared label.
Only the current container can use a private volume.

### Attach to STDIN/STDOUT/STDERR (-a)

The `-a` flag tells `docker run` to bind to the container's `STDIN`, `STDOUT`
or `STDERR`. This makes it possible to manipulate the output and input as
needed.

    $ echo "test" | docker run -i -a stdin ubuntu cat -

This pipes data into a container and prints the container's ID by attaching
only to the container's `STDIN`.

    $ docker run -a stderr ubuntu echo test

This isn't going to print anything unless there's an error because we've
only attached to the `STDERR` of the container. The container's logs
still store what's been written to `STDERR` and `STDOUT`.

    $ cat somefile | docker run -i -a stdin mybuilder dobuild

This is how piping a file into a container could be done for a build.
The container's ID will be printed after the build is done and the build
logs could be retrieved using `docker logs`. This is
useful if you need to pipe a file or something else into a container and
retrieve the container's ID once the container has finished running.

### Add host device to container (--device)

    $ docker run --device=/dev/sdc:/dev/xvdc --device=/dev/sdd --device=/dev/zero:/dev/nulo -i -t ubuntu ls -l /dev/{xvdc,sdd,nulo}
    brw-rw---- 1 root disk 8, 2 Feb  9 16:05 /dev/xvdc
    brw-rw---- 1 root disk 8, 3 Feb  9 16:05 /dev/sdd
    crw-rw-rw- 1 root root 1, 5 Feb  9 16:05 /dev/nulo

It is often necessary to directly expose devices to a container. The `--device`
option enables that. For example, a specific block storage device or loop
device or audio device can be added to an otherwise unprivileged container
(without the `--privileged` flag) and have the application directly access it.

By default, the container will be able to `read`, `write` and `mknod` these devices.
This can be overridden using a third `:rwm` set of options to each `--device`
flag:


    $ docker run --device=/dev/sda:/dev/xvdc --rm -it ubuntu fdisk  /dev/xvdc

    Command (m for help): q
    $ docker run --device=/dev/sda:/dev/xvdc:r --rm -it ubuntu fdisk  /dev/xvdc
    You will not be able to write the partition table.

    Command (m for help): q

    $ docker run --device=/dev/sda:/dev/xvdc:rw --rm -it ubuntu fdisk  /dev/xvdc

    Command (m for help): q

    $ docker run --device=/dev/sda:/dev/xvdc:m --rm -it ubuntu fdisk  /dev/xvdc
    fdisk: unable to open /dev/xvdc: Operation not permitted

> **Note:**
> `--device` cannot be safely used with ephemeral devices. Block devices
> that may be removed should not be added to untrusted containers with
> `--device`.

### Restart policies (--restart)

Use Docker's `--restart` to specify a container's *restart policy*. A restart
policy controls whether the Docker daemon restarts a container after exit.
Docker supports the following restart policies:

<table>
  <thead>
    <tr>
      <th>Policy</th>
      <th>Result</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><strong>no</strong></td>
      <td>
        Do not automatically restart the container when it exits. This is the
        default.
      </td>
    </tr>
    <tr>
      <td>
        <span style="white-space: nowrap">
          <strong>on-failure</strong>[:max-retries]
        </span>
      </td>
      <td>
        Restart only if the container exits with a non-zero exit status.
        Optionally, limit the number of restart retries the Docker
        daemon attempts.
      </td>
    </tr>
    <tr>
      <td><strong>always</strong></td>
      <td>
        Always restart the container regardless of the exit status.
        When you specify always, the Docker daemon will try to restart
        the container indefinitely. The container will also always start
        on daemon startup, regardless of the current state of the container.
      </td>
    </tr>
    <tr>
      <td><strong>unless-stopped</strong></td>
      <td>
        Always restart the container regardless of the exit status, but
        do not start it on daemon startup if the container has been put
        to a stopped state before.
      </td>
    </tr>
  </tbody>
</table>

    $ docker run --restart=always redis

This will run the `redis` container with a restart policy of **always**
so that if the container exits, Docker will restart it.

More detailed information on restart policies can be found in the
[Restart Policies (--restart)](../run.md#restart-policies-restart)
section of the Docker run reference page.

### Add entries to container hosts file (--add-host)

You can add other hosts into a container's `/etc/hosts` file by using one or
more `--add-host` flags. This example adds a static address for a host named
`docker`:

    $ docker run --add-host=docker:10.180.0.1 --rm -it debian
    $$ ping docker
    PING docker (10.180.0.1): 48 data bytes
    56 bytes from 10.180.0.1: icmp_seq=0 ttl=254 time=7.600 ms
    56 bytes from 10.180.0.1: icmp_seq=1 ttl=254 time=30.705 ms
    ^C--- docker ping statistics ---
    2 packets transmitted, 2 packets received, 0% packet loss
    round-trip min/avg/max/stddev = 7.600/19.152/30.705/11.553 ms

Sometimes you need to connect to the Docker host from within your
container. To enable this, pass the Docker host's IP address to
the container using the `--add-host` flag. To find the host's address,
use the `ip addr show` command.

The flags you pass to `ip addr show` depend on whether you are
using IPv4 or IPv6 networking in your containers. Use the following
flags for IPv4 address retrieval for a network device named `eth0`:

    $ HOSTIP=`ip -4 addr show scope global dev eth0 | grep inet | awk '{print \$2}' | cut -d / -f 1`
    $ docker run  --add-host=docker:${HOSTIP} --rm -it debian

For IPv6 use the `-6` flag instead of the `-4` flag. For other network
devices, replace `eth0` with the correct device name (for example `docker0`
for the bridge device).

### Set ulimits in container (--ulimit)

Since setting `ulimit` settings in a container requires extra privileges not
available in the default container, you can set these using the `--ulimit` flag.
`--ulimit` is specified with a soft and hard limit as such:
`<type>=<soft limit>[:<hard limit>]`, for example:

    $ docker run --ulimit nofile=1024:1024 --rm debian sh -c "ulimit -n"
    1024

> **Note:**
> If you do not provide a `hard limit`, the `soft limit` will be used
> for both values. If no `ulimits` are set, they will be inherited from
> the default `ulimits` set on the daemon.  `as` option is disabled now.
> In other words, the following script is not supported:
> `$ docker run -it --ulimit as=1024 fedora /bin/bash`

The values are sent to the appropriate `syscall` as they are set.
Docker doesn't perform any byte conversion. Take this into account when setting the values.

#### For `nproc` usage

Be careful setting `nproc` with the `ulimit` flag as `nproc` is designed by Linux to set the
maximum number of processes available to a user, not to a container.  For example, start four
containers with `daemon` user:

    docker run -d -u daemon --ulimit nproc=3 busybox top
    docker run -d -u daemon --ulimit nproc=3 busybox top
    docker run -d -u daemon --ulimit nproc=3 busybox top
    docker run -d -u daemon --ulimit nproc=3 busybox top

The 4th container fails and reports "[8] System error: resource temporarily unavailable" error.
This fails because the caller set `nproc=3` resulting in the first three containers using up
the three processes quota set for the `daemon` user.

### Stop container with signal (--stop-signal)

The `--stop-signal` flag sets the system call signal that will be sent to the container to exit.
This signal can be a valid unsigned number that matches a position in the kernel's syscall table, for instance 9,
or a signal name in the format SIGNAME, for instance SIGKILL.

### Specify isolation technology for container (--isolation)

This option is useful in situations where you are running Docker containers on
Microsoft Windows. The `--isolation <value>` option sets a container's isolation
technology. On Linux, the only supported is the `default` option which uses
Linux namespaces. These two commands are equivalent on Linux:

```
$ docker run -d busybox top
$ docker run -d --isolation default busybox top
```

On Microsoft Windows, can take any of these values:


| Value     | Description                                                                                                                                                   |
|-----------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `default` | Use the value specified by the Docker daemon's `--exec-opt` . If the `daemon` does not specify an isolation technology, Microsoft Windows uses `process` as its default value.  |
| `process` | Namespace isolation only.                                                                                                                                     |
| `hyperv`   | Hyper-V hypervisor partition-based isolation.                                                                                                                  |

On Windows, the default isolation for client is `hyperv`, and for server is
`process`. Therefore when running on Windows server without a `daemon` option 
set, these two commands are equivalent:
```
$ docker run -d --isolation default busybox top
$ docker run -d --isolation process busybox top
```

If you have set the `--exec-opt isolation=hyperv` option on the Docker `daemon`, 
if running on Windows server, any of these commands also result in `hyperv` isolation:

```
$ docker run -d --isolation default busybox top
$ docker run -d --isolation hyperv busybox top
```

### Configure namespaced kernel parameters (sysctls) at runtime

The `--sysctl` sets namespaced kernel parameters (sysctls) in the
container. For example, to turn on IP forwarding in the containers
network namespace, run this command:

    $ docker run --sysctl net.ipv4.ip_forward=1 someimage


> **Note**: Not all sysctls are namespaced. docker does not support changing sysctls
> inside of a container that also modify the host system. As the kernel 
> evolves we expect to see more sysctls become namespaced.

#### Currently supported sysctls

  `IPC Namespace`:

  kernel.msgmax, kernel.msgmnb, kernel.msgmni, kernel.sem, kernel.shmall, kernel.shmmax, kernel.shmmni, kernel.shm_rmid_forced
  Sysctls beginning with fs.mqueue.*

  If you use the `--ipc=host` option these sysctls will not be allowed.

  `Network Namespace`:
      Sysctls beginning with net.*

  If you use the `--net=host` option using these sysctls will not be allowed.

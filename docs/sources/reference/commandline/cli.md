page_title: Command Line Interface
page_description: Docker's CLI command description and usage
page_keywords: Docker, Docker documentation, CLI, command line

# Docker Command Line

{{ include "no-remote-sudo.md" }}

To list available commands, either run `docker` with no parameters
or execute `docker help`:

    $ docker
      Usage: docker [OPTIONS] COMMAND [arg...]
        -H, --host=[]: The socket(s) to bind to in daemon mode, specified using one or more tcp://host:port, unix:///path/to/socket, fd://* or fd://socketfd.

      A self-sufficient runtime for Linux containers.

      ...

Depending on your Docker system configuration, you may be required to preface
each `docker` command with `sudo`. To avoid having to use `sudo` with the
`docker` command, your system administrator can create a Unix group called
`docker` and add users to it.

For more information about installing Docker or `sudo` configuration, refer to
the [installation](/installation) instructions for your operating system.

## Environment variables

For easy reference, the following list of environment variables are supported
by the `docker` command line:

* `DOCKER_CERT_PATH` The location of your authentication keys.
* `DOCKER_DRIVER` The graph driver to use.
* `DOCKER_HOST` Daemon socket to connect to.
* `DOCKER_NOWARN_KERNEL_VERSION` Prevent warnings that your Linux kernel is
  unsuitable for Docker.
* `DOCKER_RAMDISK` If set this will disable 'pivot_root'.
* `DOCKER_TLS_VERIFY` When set Docker uses TLS and verifies the remote.
* `DOCKER_TMPDIR` Location for temporary Docker files.

Because Docker is developed using 'Go', you can also use any environment
variables used by the 'Go' runtime. In particular, you may find these useful:

* `HTTP_PROXY`
* `HTTPS_PROXY`
* `NO_PROXY`

These Go environment variables are case-insensitive. See the
[Go specification](http://golang.org/pkg/net/http/) for details on these
variables.

## Configuration files

The Docker command line stores its configuration files in a directory called
`.docker` within your `HOME` directory. Docker manages most of the files in
`.docker` and you should not modify them. However, you *can modify* the
`.docker/config.json` file to control certain aspects of how the `docker`
command behaves.

Currently, you can modify the `docker` command behavior using environment
variables or command-line options. You can also use options within
`config.json` to modify some of the same behavior. When using these
mechanisms, you must keep in mind the order of precedence among them. Command
line options override environment variables and environment variables override
properties you specify in a `config.json` file.

The `config.json` file stores a JSON encoding of a single `HttpHeaders`
property. The property specifies a set of headers to include in all messages
sent from the Docker client to the daemon. Docker does not try to interpret or
understand these header; it simply puts them into the messages. Docker does
not allow these headers to change any headers it sets for itself.

Following is a sample `config.json` file:

    {
      "HttpHeaders: {
        "MyHeader": "MyValue"
      }
    }

## Help

To list the help on any command just execute the command, followed by the
`--help` option.

    $ docker run --help

    Usage: docker run [OPTIONS] IMAGE [COMMAND] [ARG...]

    Run a command in a new container

      -a, --attach=[]            Attach to STDIN, STDOUT or STDERR
      -c, --cpu-shares=0         CPU shares (relative weight)
    ...

## Option types

Single character command line options can be combined, so rather than
typing `docker run -i -t --name test busybox sh`,
you can write `docker run -it --name test busybox sh`.

### Boolean

Boolean options take the form `-d=false`. The value you see in the help text is
the default value which is set if you do **not** specify that flag. If you
specify a Boolean flag without a value, this will set the flag to `true`,
irrespective of the default value.

For example, running `docker run -d` will set the value to `true`, so your
container **will** run in "detached" mode, in the background.

Options which default to `true` (e.g., `docker build --rm=true`) can only be
set to the non-default value by explicitly setting them to `false`:

    $ docker build --rm=false .

### Multi

You can specify options like `-a=[]` multiple times in a single command line,
for example in these commands:

    $ docker run -a stdin -a stdout -i -t ubuntu /bin/bash
    $ docker run -a stdin -a stdout -a stderr ubuntu /bin/ls

Sometimes, multiple options can call for a more complex value string as for
`-v`:

    $ docker run -v /host:/container example/mysql

> **Note:**
> Do not use the `-t` and `-a stderr` options together due to
> limitations in the `pty` implementation. All `stderr` in `pty` mode
> simply goes to `stdout`.

### Strings and Integers

Options like `--name=""` expect a string, and they
can only be specified once. Options like `-c=0`
expect an integer, and they can only be specified once.

## daemon

    Usage: docker [OPTIONS] COMMAND [arg...]

    A self-sufficient runtime for linux containers.

    Options:
      --api-cors-header=""                   Set CORS headers in the remote API
      -b, --bridge=""                        Attach containers to a network bridge
      --bip=""                               Specify network bridge IP
      -D, --debug=false                      Enable debug mode
      -d, --daemon=false                     Enable daemon mode
      --default-gateway=""                   Container default gateway IPv4 address
      --default-gateway-v6=""                Container default gateway IPv6 address
      --dns=[]                               DNS server to use
      --dns-search=[]                        DNS search domains to use
      --default-ulimit=[]                    Set default ulimit settings for containers
      -e, --exec-driver="native"             Exec driver to use
      --exec-opt=[]                          Set exec driver options
      --exec-root="/var/run/docker"          Root of the Docker execdriver
      --fixed-cidr=""                        IPv4 subnet for fixed IPs
      --fixed-cidr-v6=""                     IPv6 subnet for fixed IPs
      -G, --group="docker"                   Group for the unix socket
      -g, --graph="/var/lib/docker"          Root of the Docker runtime
      -H, --host=[]                          Daemon socket(s) to connect to
      -h, --help=false                       Print usage
      --icc=true                             Enable inter-container communication
      --insecure-registry=[]                 Enable insecure registry communication
      --ip=0.0.0.0                           Default IP when binding container ports
      --ip-forward=true                      Enable net.ipv4.ip_forward
      --ip-masq=true                         Enable IP masquerading
      --iptables=true                        Enable addition of iptables rules
      --ipv6=false                           Enable IPv6 networking
      -l, --log-level="info"                 Set the logging level
      --label=[]                             Set key=value labels to the daemon
      --log-driver="json-file"               Default driver for container logs
      --log-opt=[]                           Log driver specific options
      --mtu=0                                Set the containers network MTU
      -p, --pidfile="/var/run/docker.pid"    Path to use for daemon PID file
      --registry-mirror=[]                   Preferred Docker registry mirror
      -s, --storage-driver=""                Storage driver to use
      --selinux-enabled=false                Enable selinux support
      --storage-opt=[]                       Set storage driver options
      --tls=false                            Use TLS; implied by --tlsverify
      --tlscacert="~/.docker/ca.pem"         Trust certs signed only by this CA
      --tlscert="~/.docker/cert.pem"         Path to TLS certificate file
      --tlskey="~/.docker/key.pem"           Path to TLS key file
      --tlsverify=false                      Use TLS and verify the remote
      --userland-proxy=true                  Use userland proxy for loopback traffic
      -v, --version=false                    Print version information and quit

Options with [] may be specified multiple times.

The Docker daemon is the persistent process that manages containers. Docker
uses the same binary for both the daemon and client. To run the daemon you
provide the `-d` flag.

To run the daemon with debug output, use `docker -d -D`.

### Daemon socket option

The Docker daemon can listen for [Docker Remote API](/reference/api/docker_remote_api/)
requests via three different types of Socket: `unix`, `tcp`, and `fd`.

By default, a `unix` domain socket (or IPC socket) is created at
`/var/run/docker.sock`, requiring either `root` permission, or `docker` group
membership.

If you need to access the Docker daemon remotely, you need to enable the `tcp`
Socket. Beware that the default setup provides un-encrypted and
un-authenticated direct access to the Docker daemon - and should be secured
either using the [built in HTTPS encrypted socket](/articles/https/), or by
putting a secure web proxy in front of it. You can listen on port `2375` on all
network interfaces with `-H tcp://0.0.0.0:2375`, or on a particular network
interface using its IP address: `-H tcp://192.168.59.103:2375`. It is
conventional to use port `2375` for un-encrypted, and port `2376` for encrypted
communication with the daemon.

> **Note:**
> If you're using an HTTPS encrypted socket, keep in mind that only
> TLS1.0 and greater are supported. Protocols SSLv3 and under are not
> supported anymore for security reasons.

On Systemd based systems, you can communicate with the daemon via
[Systemd socket activation](http://0pointer.de/blog/projects/socket-activation.html),
use `docker -d -H fd://`. Using `fd://` will work perfectly for most setups but
you can also specify individual sockets: `docker -d -H fd://3`. If the
specified socket activated files aren't found, then Docker will exit. You can
find examples of using Systemd socket activation with Docker and Systemd in the
[Docker source tree](https://github.com/docker/docker/tree/master/contrib/init/systemd/).

You can configure the Docker daemon to listen to multiple sockets at the same
time using multiple `-H` options:

    # listen using the default unix socket, and on 2 specific IP addresses on this host.
    docker -d -H unix:///var/run/docker.sock -H tcp://192.168.59.106 -H tcp://10.10.10.2

The Docker client will honor the `DOCKER_HOST` environment variable to set the
`-H` flag for the client.

    $ docker -H tcp://0.0.0.0:2375 ps
    # or
    $ export DOCKER_HOST="tcp://0.0.0.0:2375"
    $ docker ps
    # both are equal

Setting the `DOCKER_TLS_VERIFY` environment variable to any value other than
the empty string is equivalent to setting the `--tlsverify` flag. The following
are equivalent:

    $ docker --tlsverify ps
    # or
    $ export DOCKER_TLS_VERIFY=1
    $ docker ps

The Docker client will honor the `HTTP_PROXY`, `HTTPS_PROXY`, and `NO_PROXY`
environment variables (or the lowercase versions thereof). `HTTPS_PROXY` takes
precedence over `HTTP_PROXY`.

### Daemon storage-driver option

The Docker daemon has support for several different image layer storage
drivers: `aufs`, `devicemapper`, `btrfs`, `zfs` and `overlay`.

The `aufs` driver is the oldest, but is based on a Linux kernel patch-set that
is unlikely to be merged into the main kernel. These are also known to cause
some serious kernel crashes. However, `aufs` is also the only storage driver
that allows containers to share executable and shared library memory, so is a
useful choice when running thousands of containers with the same program or
libraries.

The `devicemapper` driver uses thin provisioning and Copy on Write (CoW)
snapshots. For each devicemapper graph location – typically
`/var/lib/docker/devicemapper` – a thin pool is created based on two block
devices, one for data and one for metadata. By default, these block devices
are created automatically by using loopback mounts of automatically created
sparse files. Refer to [Storage driver options](#storage-driver-options) below
for a way how to customize this setup.
[~jpetazzo/Resizing Docker containers with the Device Mapper plugin](http://jpetazzo.github.io/2014/01/29/docker-device-mapper-resize/)
article explains how to tune your existing setup without the use of options.

The `btrfs` driver is very fast for `docker build` - but like `devicemapper`
does not share executable memory between devices. Use
`docker -d -s btrfs -g /mnt/btrfs_partition`.

The `zfs` driver is probably not fast as `btrfs` but has a longer track record
on stability. Thanks to `Single Copy ARC` shared blocks between clones will be
cached only once. Use `docker -d -s zfs`. To select a different zfs filesystem
set `zfs.fsname` option as described in [Storage driver options](#storage-driver-options).

The `overlay` is a very fast union filesystem. It is now merged in the main
Linux kernel as of [3.18.0](https://lkml.org/lkml/2014/10/26/137). Call
`docker -d -s overlay` to use it.

> **Note:**
> As promising as `overlay` is, the feature is still quite young and should not
> be used in production. Most notably, using `overlay` can cause excessive
> inode consumption (especially as the number of images grows), as well as
> being incompatible with the use of RPMs.

> **Note:**
> It is currently unsupported on `btrfs` or any Copy on Write filesystem
> and should only be used over `ext4` partitions.

#### Storage driver options

Particular storage-driver can be configured with options specified with
`--storage-opt` flags. Options for `devicemapper` are prefixed with `dm` and
options for `zfs` start with `zfs`.

Currently supported options of `devicemapper`:

 *  `dm.basesize`

    Specifies the size to use when creating the base device, which limits the
    size of images and containers. The default value is 10G. Note, thin devices
    are inherently "sparse", so a 10G device which is mostly empty doesn't use
    10 GB of space on the pool. However, the filesystem will use more space for
    the empty case the larger the device is.

     **Warning:** This value affects the system-wide "base" empty filesystem
     that may already be initialized and inherited by pulled images. Typically,
     a change to this value will require additional steps to take effect:

        $ sudo service docker stop
        $ sudo rm -rf /var/lib/docker
        $ sudo service docker start

    Example use:

        $ docker -d --storage-opt dm.basesize=20G

 *  `dm.loopdatasize`

    Specifies the size to use when creating the loopback file for the "data"
    device which is used for the thin pool. The default size is 100G. Note that
    the file is sparse, so it will not initially take up this much space.

    Example use:

        $ docker -d --storage-opt dm.loopdatasize=200G

 *  `dm.loopmetadatasize`

    Specifies the size to use when creating the loopback file for the
    "metadata" device which is used for the thin pool. The default size is 2G.
    Note that the file is sparse, so it will not initially take up this much
    space.

    Example use:

        $ docker -d --storage-opt dm.loopmetadatasize=4G

 *  `dm.fs`

    Specifies the filesystem type to use for the base device. The supported
    options are "ext4" and "xfs". The default is "ext4"

    Example use:

        $ docker -d --storage-opt dm.fs=xfs

 *  `dm.mkfsarg`

    Specifies extra mkfs arguments to be used when creating the base device.

    Example use:

        $ docker -d --storage-opt "dm.mkfsarg=-O ^has_journal"

 *  `dm.mountopt`

    Specifies extra mount options used when mounting the thin devices.

    Example use:

        $ docker -d --storage-opt dm.mountopt=nodiscard

 *  `dm.datadev`

    Specifies a custom blockdevice to use for data for the thin pool.

    If using a block device for device mapper storage, ideally both datadev and
    metadatadev should be specified to completely avoid using the loopback
    device.

    Example use:

        $ docker -d \
            --storage-opt dm.datadev=/dev/sdb1 \
            --storage-opt dm.metadatadev=/dev/sdc1

 *  `dm.metadatadev`

    Specifies a custom blockdevice to use for metadata for the thin pool.

    For best performance the metadata should be on a different spindle than the
    data, or even better on an SSD.

    If setting up a new metadata pool it is required to be valid. This can be
    achieved by zeroing the first 4k to indicate empty metadata, like this:

        $ dd if=/dev/zero of=$metadata_dev bs=4096 count=1

    Example use:

        $ docker -d \
            --storage-opt dm.datadev=/dev/sdb1 \
            --storage-opt dm.metadatadev=/dev/sdc1

 *  `dm.blocksize`

    Specifies a custom blocksize to use for the thin pool. The default
    blocksize is 64K.

    Example use:

        $ docker -d --storage-opt dm.blocksize=512K

 *  `dm.blkdiscard`

    Enables or disables the use of blkdiscard when removing devicemapper
    devices. This is enabled by default (only) if using loopback devices and is
    required to resparsify the loopback file on image/container removal.

    Disabling this on loopback can lead to *much* faster container removal
    times, but will make the space used in `/var/lib/docker` directory not be
    returned to the system for other use when containers are removed.

    Example use:

        $ docker -d --storage-opt dm.blkdiscard=false

 *  `dm.override_udev_sync_check`

    Overrides the `udev` synchronization checks between `devicemapper` and `udev`.
    `udev` is the device manager for the Linux kernel.

    To view the `udev` sync support of a Docker daemon that is using the
    `devicemapper` driver, run:

        $ docker info
        [...]
        Udev Sync Supported: true
        [...]

    When `udev` sync support is `true`, then `devicemapper` and udev can
    coordinate the activation and deactivation of devices for containers.

    When `udev` sync support is `false`, a race condition occurs between
    the`devicemapper` and `udev` during create and cleanup. The race condition
    results in errors and failures. (For information on these failures, see
    [docker#4036](https://github.com/docker/docker/issues/4036))

    To allow the `docker` daemon to start, regardless of `udev` sync not being
    supported, set `dm.override_udev_sync_check` to true:

        $ docker -d --storage-opt dm.override_udev_sync_check=true

    When this value is `true`, the  `devicemapper` continues and simply warns
    you the errors are happening.

    > **Note:**
    > The ideal is to pursue a `docker` daemon and environment that does
    > support synchronizing with `udev`. For further discussion on this
    > topic, see [docker#4036](https://github.com/docker/docker/issues/4036).
    > Otherwise, set this flag for migrating existing Docker daemons to
    > a daemon with a supported environment.

### Docker execdriver option

Currently supported options of `zfs`:

 * `zfs.fsname`

    Set zfs filesystem under which docker will create its own datasets.
    By default docker will pick up the zfs filesystem where docker graph
    (`/var/lib/docker`) is located.

    Example use:

        $ docker -d -s zfs --storage-opt zfs.fsname=zroot/docker

The Docker daemon uses a specifically built `libcontainer` execution driver as
its interface to the Linux kernel `namespaces`, `cgroups`, and `SELinux`.

There is still legacy support for the original [LXC userspace tools](
https://linuxcontainers.org/) via the `lxc` execution driver, however, this is
not where the primary development of new functionality is taking place.
Add `-e lxc` to the daemon flags to use the `lxc` execution driver.

#### Options for the native execdriver

You can configure the `native` (libcontainer) execdriver using options specified
with the `--exec-opt` flag. All the flag's options have the `native` prefix. A
single `native.cgroupdriver` option is available.

The `native.cgroupdriver` option specifies the management of the container's
cgroups. You can specify `cgroupfs` or `systemd`. If you specify `systemd` and
it is not available, the system uses `cgroupfs`. By default, if no option is
specified, the execdriver first tries `systemd` and falls back to `cgroupfs`.
This example sets the execdriver to `cgroupfs`:

    $ sudo docker -d --exec-opt native.cgroupdriver=cgroupfs

Setting this option applies to all containers the daemon launches.

### Daemon DNS options

To set the DNS server for all Docker containers, use
`docker -d --dns 8.8.8.8`.

To set the DNS search domain for all Docker containers, use
`docker -d --dns-search example.com`.

### Insecure registries

Docker considers a private registry either secure or insecure. In the rest of
this section, *registry* is used for *private registry*, and `myregistry:5000`
is a placeholder example for a private registry.

A secure registry uses TLS and a copy of its CA certificate is placed on the
Docker host at `/etc/docker/certs.d/myregistry:5000/ca.crt`. An insecure
registry is either not using TLS (i.e., listening on plain text HTTP), or is
using TLS with a CA certificate not known by the Docker daemon. The latter can
happen when the certificate was not found under
`/etc/docker/certs.d/myregistry:5000/`, or if the certificate verification
failed (i.e., wrong CA).

By default, Docker assumes all, but local (see local registries below),
registries are secure. Communicating with an insecure registry is not possible
if Docker assumes that registry is secure. In order to communicate with an
insecure registry, the Docker daemon requires `--insecure-registry` in one of
the following two forms:

* `--insecure-registry myregistry:5000` tells the Docker daemon that
  myregistry:5000 should be considered insecure.
* `--insecure-registry 10.1.0.0/16` tells the Docker daemon that all registries
  whose domain resolve to an IP address is part of the subnet described by the
  CIDR syntax, should be considered insecure.

The flag can be used multiple times to allow multiple registries to be marked
as insecure.

If an insecure registry is not marked as insecure, `docker pull`,
`docker push`, and `docker search` will result in an error message prompting
the user to either secure or pass the `--insecure-registry` flag to the Docker
daemon as described above.

Local registries, whose IP address falls in the 127.0.0.0/8 range, are
automatically marked as insecure as of Docker 1.3.2. It is not recommended to
rely on this, as it may change in the future.

### Running a Docker daemon behind a HTTPS_PROXY

When running inside a LAN that uses a `HTTPS` proxy, the Docker Hub
certificates will be replaced by the proxy's certificates. These certificates
need to be added to your Docker host's configuration:

1. Install the `ca-certificates` package for your distribution
2. Ask your network admin for the proxy's CA certificate and append them to
   `/etc/pki/tls/certs/ca-bundle.crt`
3. Then start your Docker daemon with `HTTPS_PROXY=http://username:password@proxy:port/ docker -d`.
   The `username:` and `password@` are optional - and are only needed if your
   proxy is set up to require authentication.

This will only add the proxy and authentication to the Docker daemon's requests -
your `docker build`s and running containers will need extra configuration to
use the proxy

### Default Ulimits

`--default-ulimit` allows you to set the default `ulimit` options to use for
all containers. It takes the same options as `--ulimit` for `docker run`. If
these defaults are not set, `ulimit` settings will be inherited, if not set on
`docker run`, from the Docker daemon. Any `--ulimit` options passed to 
`docker run` will overwrite these defaults.

### Miscellaneous options

IP masquerading uses address translation to allow containers without a public
IP to talk to other machines on the Internet. This may interfere with some
network topologies and can be disabled with --ip-masq=false.

Docker supports softlinks for the Docker data directory (`/var/lib/docker`) and
for `/var/lib/docker/tmp`. The `DOCKER_TMPDIR` and the data directory can be
set like this:

    DOCKER_TMPDIR=/mnt/disk2/tmp /usr/local/bin/docker -d -D -g /var/lib/docker -H unix:// > /var/lib/boot2docker/docker.log 2>&1
    # or
    export DOCKER_TMPDIR=/mnt/disk2/tmp
    /usr/local/bin/docker -d -D -g /var/lib/docker -H unix:// > /var/lib/boot2docker/docker.log 2>&1

## attach

    Usage: docker attach [OPTIONS] CONTAINER

    Attach to a running container

      --no-stdin=false    Do not attach STDIN
      --sig-proxy=true    Proxy all received signals to the process

The `docker attach` command allows you to attach to a running container using
the container's ID or name, either to view its ongoing output or to control it
interactively. You can attach to the same contained process multiple times
simultaneously, screen sharing style, or quickly view the progress of your
daemonized process.

You can detach from the container and leave it running with `CTRL-p CTRL-q`
(for a quiet exit) or with `CTRL-c` if `--sig-proxy` is false.

If `--sig-proxy` is true (the default),`CTRL-c` sends a `SIGINT` to the
container.

> **Note:**
> A process running as PID 1 inside a container is treated specially by
> Linux: it ignores any signal with the default action. So, the process
> will not terminate on `SIGINT` or `SIGTERM` unless it is coded to do
> so.

It is forbidden to redirect the standard input of a `docker attach` command
while attaching to a tty-enabled container (i.e.: launched with `-t`).

#### Examples

    $ docker run -d --name topdemo ubuntu /usr/bin/top -b
    $ docker attach topdemo
    top - 02:05:52 up  3:05,  0 users,  load average: 0.01, 0.02, 0.05
    Tasks:   1 total,   1 running,   0 sleeping,   0 stopped,   0 zombie
    Cpu(s):  0.1%us,  0.2%sy,  0.0%ni, 99.7%id,  0.0%wa,  0.0%hi,  0.0%si,  0.0%st
    Mem:    373572k total,   355560k used,    18012k free,    27872k buffers
    Swap:   786428k total,        0k used,   786428k free,   221740k cached

    PID USER      PR  NI  VIRT  RES  SHR S %CPU %MEM    TIME+  COMMAND
     1 root      20   0 17200 1116  912 R    0  0.3   0:00.03 top

     top - 02:05:55 up  3:05,  0 users,  load average: 0.01, 0.02, 0.05
     Tasks:   1 total,   1 running,   0 sleeping,   0 stopped,   0 zombie
     Cpu(s):  0.0%us,  0.2%sy,  0.0%ni, 99.8%id,  0.0%wa,  0.0%hi,  0.0%si,  0.0%st
     Mem:    373572k total,   355244k used,    18328k free,    27872k buffers
     Swap:   786428k total,        0k used,   786428k free,   221776k cached

       PID USER      PR  NI  VIRT  RES  SHR S %CPU %MEM    TIME+  COMMAND
           1 root      20   0 17208 1144  932 R    0  0.3   0:00.03 top


     top - 02:05:58 up  3:06,  0 users,  load average: 0.01, 0.02, 0.05
     Tasks:   1 total,   1 running,   0 sleeping,   0 stopped,   0 zombie
     Cpu(s):  0.2%us,  0.3%sy,  0.0%ni, 99.5%id,  0.0%wa,  0.0%hi,  0.0%si,  0.0%st
     Mem:    373572k total,   355780k used,    17792k free,    27880k buffers
     Swap:   786428k total,        0k used,   786428k free,   221776k cached

     PID USER      PR  NI  VIRT  RES  SHR S %CPU %MEM    TIME+  COMMAND
          1 root      20   0 17208 1144  932 R    0  0.3   0:00.03 top
    ^C$
    $ echo $?
    0
    $ docker ps -a | grep topdemo
    7998ac8581f9        ubuntu:14.04        "/usr/bin/top -b"   38 seconds ago      Exited (0) 21 seconds ago                          topdemo

And in this second example, you can see the exit code returned by the `bash`
process is returned by the `docker attach` command to its caller too:

    $ docker run --name test -d -it debian
    275c44472aebd77c926d4527885bb09f2f6db21d878c75f0a1c212c03d3bcfab
    $ docker attach test
    $$ exit 13
    exit
    $ echo $?
    13
    $ docker ps -a | grep test
    275c44472aeb        debian:7            "/bin/bash"         26 seconds ago      Exited (13) 17 seconds ago                         test

## build

    Usage: docker build [OPTIONS] PATH | URL | -

    Build a new image from the source code at PATH

      -f, --file=""            Name of the Dockerfile (Default is 'PATH/Dockerfile')
      --force-rm=false         Always remove intermediate containers
      --no-cache=false         Do not use cache when building the image
      --pull=false             Always attempt to pull a newer version of the image
      -q, --quiet=false        Suppress the verbose output generated by the containers
      --rm=true                Remove intermediate containers after a successful build
      -t, --tag=""             Repository name (and optionally a tag) for the image
      -m, --memory=""          Memory limit for all build containers
      --memory-swap=""         Total memory (memory + swap), `-1` to disable swap
      -c, --cpu-shares         CPU Shares (relative weight)
      --cpuset-mems=""         MEMs in which to allow execution, e.g. `0-3`, `0,1`
      --cpuset-cpus=""         CPUs in which to allow execution, e.g. `0-3`, `0,1`
      --cgroup-parent=""       Optional parent cgroup for the container

Builds Docker images from a Dockerfile and a "context". A build's context is
the files located in the specified `PATH` or `URL`. The build process can refer
to any of the files in the context. For example, your build can use an
[*ADD*](/reference/builder/#add) instruction to reference a file in the
context.

The `URL` parameter can specify the location of a Git repository; the repository
acts as the build context. The system recursively clones the repository and its
submodules using a `git clone --depth 1 --recursive` command. This command runs
in a temporary directory on your local host. After the command succeeds, the
directory is sent to the Docker daemon as the context. Local clones give you the
ability to access private repositories using local user credentials, VPNs, and
so forth.

Git URLs accept context configuration in their fragment section, separated by a
colon `:`.  The first part represents the reference that Git will check out,
this can be either a branch, a tag, or a commit SHA. The second part represents
a subdirectory inside the repository that will be used as a build context.

For example, run this command to use a directory called `docker` in the branch
`container`:

      $ docker build https://github.com/docker/rootfs.git#container:docker

The following table represents all the valid suffixes with their build
contexts:

Build Syntax Suffix | Commit Used | Build Context Used
--------------------|-------------|-------------------
`myrepo.git` | `refs/heads/master` | `/`
`myrepo.git#mytag` | `refs/tags/mytag` | `/`
`myrepo.git#mybranch` | `refs/heads/mybranch` | `/`
`myrepo.git#abcdef` | `sha1 = abcdef` | `/`
`myrepo.git#:myfolder` | `refs/heads/master` | `/myfolder`
`myrepo.git#master:myfolder` | `refs/heads/master` | `/myfolder`
`myrepo.git#mytag:myfolder` | `refs/tags/mytag` | `/myfolder`
`myrepo.git#mybranch:myfolder` | `refs/heads/mybranch` | `/myfolder`
`myrepo.git#abcdef:myfolder` | `sha1 = abcdef` | `/myfolder`

Instead of specifying a context, you can pass a single Dockerfile in the `URL`
or pipe the file in via `STDIN`. To pipe a Dockerfile from `STDIN`:

    docker build - < Dockerfile

If you use STDIN or specify a `URL`, the system places the contents into a file
called `Dockerfile`, and any `-f`, `--file` option is ignored. In this
scenario, there is no context.

By default the `docker build` command will look for a `Dockerfile` at the root
of the build context. The `-f`, `--file`, option lets you specify the path to
an alternative file to use instead. This is useful in cases where the same set
of files are used for multiple builds. The path must be to a file within the
build context. If a relative path is specified then it must to be relative to
the current directory.

In most cases, it's best to put each Dockerfile in an empty directory. Then,
add to that directory only the files needed for building the Dockerfile. To
increase the build's performance, you can exclude files and directories by
adding a `.dockerignore` file to that directory as well. For information on
creating one, see the [.dockerignore file](../../reference/builder/#dockerignore-file).

If the Docker client loses connection to the daemon, the build is canceled.
This happens if you interrupt the Docker client with `ctrl-c` or if the Docker
client is killed for any reason.

> **Note:**
> Currently only the "run" phase of the build can be canceled until pull
> cancellation is implemented).

### Return code

On a successful build, a return code of success `0` will be returned.  When the
build fails, a non-zero failure code will be returned.

There should be informational output of the reason for failure output to
`STDERR`:

    $ docker build -t fail .
    Sending build context to Docker daemon 2.048 kB
    Sending build context to Docker daemon
    Step 0 : FROM busybox
     ---> 4986bf8c1536
    Step 1 : RUN exit 13
     ---> Running in e26670ec7a0a
    INFO[0000] The command [/bin/sh -c exit 13] returned a non-zero code: 13
    $ echo $?
    1

See also:

[*Dockerfile Reference*](/reference/builder).

### Examples

    $ docker build .
    Uploading context 10240 bytes
    Step 1 : FROM busybox
    Pulling repository busybox
     ---> e9aa60c60128MB/2.284 MB (100%) endpoint: https://cdn-registry-1.docker.io/v1/
    Step 2 : RUN ls -lh /
     ---> Running in 9c9e81692ae9
    total 24
    drwxr-xr-x    2 root     root        4.0K Mar 12  2013 bin
    drwxr-xr-x    5 root     root        4.0K Oct 19 00:19 dev
    drwxr-xr-x    2 root     root        4.0K Oct 19 00:19 etc
    drwxr-xr-x    2 root     root        4.0K Nov 15 23:34 lib
    lrwxrwxrwx    1 root     root           3 Mar 12  2013 lib64 -> lib
    dr-xr-xr-x  116 root     root           0 Nov 15 23:34 proc
    lrwxrwxrwx    1 root     root           3 Mar 12  2013 sbin -> bin
    dr-xr-xr-x   13 root     root           0 Nov 15 23:34 sys
    drwxr-xr-x    2 root     root        4.0K Mar 12  2013 tmp
    drwxr-xr-x    2 root     root        4.0K Nov 15 23:34 usr
     ---> b35f4035db3f
    Step 3 : CMD echo Hello world
     ---> Running in 02071fceb21b
     ---> f52f38b7823e
    Successfully built f52f38b7823e
    Removing intermediate container 9c9e81692ae9
    Removing intermediate container 02071fceb21b

This example specifies that the `PATH` is `.`, and so all the files in the
local directory get `tar`d and sent to the Docker daemon. The `PATH` specifies
where to find the files for the "context" of the build on the Docker daemon.
Remember that the daemon could be running on a remote machine and that no
parsing of the Dockerfile happens at the client side (where you're running
`docker build`). That means that *all* the files at `PATH` get sent, not just
the ones listed to [*ADD*](/reference/builder/#add) in the Dockerfile.

The transfer of context from the local machine to the Docker daemon is what the
`docker` client means when you see the "Sending build context" message.

If you wish to keep the intermediate containers after the build is complete,
you must use `--rm=false`. This does not affect the build cache.

    $ docker build .
    Uploading context 18.829 MB
    Uploading context
    Step 0 : FROM busybox
     ---> 769b9341d937
    Step 1 : CMD echo Hello world
     ---> Using cache
     ---> 99cc1ad10469
    Successfully built 99cc1ad10469
    $ echo ".git" > .dockerignore
    $ docker build .
    Uploading context  6.76 MB
    Uploading context
    Step 0 : FROM busybox
     ---> 769b9341d937
    Step 1 : CMD echo Hello world
     ---> Using cache
     ---> 99cc1ad10469
    Successfully built 99cc1ad10469

This example shows the use of the `.dockerignore` file to exclude the `.git`
directory from the context. Its effect can be seen in the changed size of the
uploaded context. The builder reference contains detailed information on
[creating a .dockerignore file](../../builder/#dockerignore-file)

    $ docker build -t vieux/apache:2.0 .

This will build like the previous example, but it will then tag the resulting
image. The repository name will be `vieux/apache` and the tag will be `2.0`

    $ docker build - < Dockerfile

This will read a Dockerfile from `STDIN` without context. Due to the lack of a
context, no contents of any local directory will be sent to the Docker daemon.
Since there is no context, a Dockerfile `ADD` only works if it refers to a
remote URL.

    $ docker build - < context.tar.gz

This will build an image for a compressed context read from `STDIN`.  Supported
formats are: bzip2, gzip and xz.

    $ docker build github.com/creack/docker-firefox

This will clone the GitHub repository and use the cloned repository as context.
The Dockerfile at the root of the repository is used as Dockerfile. Note that
you can specify an arbitrary Git repository by using the `git://` or `git@`
schema.

    $ docker build -f Dockerfile.debug .

This will use a file called `Dockerfile.debug` for the build instructions
instead of `Dockerfile`.

    $ docker build -f dockerfiles/Dockerfile.debug -t myapp_debug .
    $ docker build -f dockerfiles/Dockerfile.prod  -t myapp_prod .

The above commands will build the current build context (as specified by the
`.`) twice, once using a debug version of a `Dockerfile` and once using a
production version.

    $ cd /home/me/myapp/some/dir/really/deep
    $ docker build -f /home/me/myapp/dockerfiles/debug /home/me/myapp
    $ docker build -f ../../../../dockerfiles/debug /home/me/myapp

These two `docker build` commands do the exact same thing. They both use the
contents of the `debug` file instead of looking for a `Dockerfile` and will use
`/home/me/myapp` as the root of the build context. Note that `debug` is in the
directory structure of the build context, regardless of how you refer to it on
the command line.

> **Note:**
> `docker build` will return a `no such file or directory` error if the
> file or directory does not exist in the uploaded context. This may
> happen if there is no context, or if you specify a file that is
> elsewhere on the Host system. The context is limited to the current
> directory (and its children) for security reasons, and to ensure
> repeatable builds on remote Docker hosts. This is also the reason why
> `ADD ../file` will not work.

When `docker build` is run with the `--cgroup-parent` option the containers
used in the build will be run with the [corresponding `docker run`
flag](/reference/run/#specifying-custom-cgroups).

## commit

    Usage: docker commit [OPTIONS] CONTAINER [REPOSITORY[:TAG]]

    Create a new image from a container's changes

      -a, --author=""     Author (e.g., "John Hannibal Smith <hannibal@a-team.com>")
      -c, --change=[]     Apply specified Dockerfile instructions while committing the image
      -m, --message=""    Commit message
      -p, --pause=true    Pause container during commit

It can be useful to commit a container's file changes or settings into a new
image. This allows you debug a container by running an interactive shell, or to
export a working dataset to another server. Generally, it is better to use
Dockerfiles to manage your images in a documented and maintainable way.

By default, the container being committed and its processes will be paused
while the image is committed. This reduces the likelihood of encountering data
corruption during the process of creating the commit.  If this behavior is
undesired, set the 'p' option to false.

The `--change` option will apply `Dockerfile` instructions to the image that is
created.  Supported `Dockerfile` instructions:
`CMD`|`ENTRYPOINT`|`ENV`|`EXPOSE`|`ONBUILD`|`USER`|`VOLUME`|`WORKDIR`

#### Commit a container

    $ docker ps
    ID                  IMAGE               COMMAND             CREATED             STATUS              PORTS
    c3f279d17e0a        ubuntu:12.04        /bin/bash           7 days ago          Up 25 hours
    197387f1b436        ubuntu:12.04        /bin/bash           7 days ago          Up 25 hours
    $ docker commit c3f279d17e0a  SvenDowideit/testimage:version3
    f5283438590d
    $ docker images
    REPOSITORY                        TAG                 ID                  CREATED             VIRTUAL SIZE
    SvenDowideit/testimage            version3            f5283438590d        16 seconds ago      335.7 MB

#### Commit a container with new configurations

    $ docker ps
    ID                  IMAGE               COMMAND             CREATED             STATUS              PORTS
    c3f279d17e0a        ubuntu:12.04        /bin/bash           7 days ago          Up 25 hours
    197387f1b436        ubuntu:12.04        /bin/bash           7 days ago          Up 25 hours
    $ docker inspect -f "{{ .Config.Env }}" c3f279d17e0a
    [HOME=/ PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin]
    $ docker commit --change "ENV DEBUG true" c3f279d17e0a  SvenDowideit/testimage:version3
    f5283438590d
    $ docker inspect -f "{{ .Config.Env }}" f5283438590d
    [HOME=/ PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin DEBUG=true]

## cp

Copy files or folders from a container's filesystem to the directory on the
host. Use '-' to write the data as a tar file to `STDOUT`. `CONTAINER:PATH` is
relative to the root of the container's filesystem.

    Usage: docker cp CONTAINER:PATH HOSTDIR|-

    Copy files/folders from the PATH to the HOSTDIR.


## create

Creates a new container.

    Usage: docker create [OPTIONS] IMAGE [COMMAND] [ARG...]

    Create a new container

      -a, --attach=[]            Attach to STDIN, STDOUT or STDERR
      --add-host=[]              Add a custom host-to-IP mapping (host:ip)
      --blkio-weight=0           Block IO weight (relative weight)
      -c, --cpu-shares=0         CPU shares (relative weight)
      --cap-add=[]               Add Linux capabilities
      --cap-drop=[]              Drop Linux capabilities
      --cgroup-parent=""         Optional parent cgroup for the container
      --cidfile=""               Write the container ID to the file
      --cpuset-cpus=""           CPUs in which to allow execution (0-3, 0,1)
      --cpuset-mems=""           Memory nodes (MEMs) in which to allow execution (0-3, 0,1)
      --cpu-period=0             Limit the CPU CFS (Completely Fair Scheduler) period
      --cpu-quota=0              Limit the CPU CFS (Completely Fair Scheduler) quota
      --device=[]                Add a host device to the container
      --dns=[]                   Set custom DNS servers
      --dns-search=[]            Set custom DNS search domains
      -e, --env=[]               Set environment variables
      --entrypoint=""            Overwrite the default ENTRYPOINT of the image
      --env-file=[]              Read in a file of environment variables
      --expose=[]                Expose a port or a range of ports
      -h, --hostname=""          Container host name
      -i, --interactive=false    Keep STDIN open even if not attached
      --ipc=""                   IPC namespace to use
      -l, --label=[]             Set metadata on the container (e.g., --label=com.example.key=value)
      --label-file=[]            Read in a line delimited file of labels
      --link=[]                  Add link to another container
      --log-driver=""            Logging driver for container
      --log-opt=[]               Log driver specific options
      --lxc-conf=[]              Add custom lxc options
      -m, --memory=""            Memory limit
      --mac-address=""           Container MAC address (e.g. 92:d0:c6:0a:29:33)
      --name=""                  Assign a name to the container
      --net="bridge"             Set the Network mode for the container
      --oom-kill-disable=false   Whether to disable OOM Killer for the container or not
      -P, --publish-all=false    Publish all exposed ports to random ports
      -p, --publish=[]           Publish a container's port(s) to the host
      --pid=""                   PID namespace to use
      --uts=""                   UTS namespace to use
      --privileged=false         Give extended privileges to this container
      --read-only=false          Mount the container's root filesystem as read only
      --restart="no"             Restart policy (no, on-failure[:max-retry], always)
      --security-opt=[]          Security options
      -t, --tty=false            Allocate a pseudo-TTY
      -u, --user=""              Username or UID
      -v, --volume=[]            Bind mount a volume
      --volumes-from=[]          Mount volumes from the specified container(s)
      -w, --workdir=""           Working directory inside the container

The `docker create` command creates a writeable container layer over the
specified image and prepares it for running the specified command.  The
container ID is then printed to `STDOUT`.  This is similar to `docker run -d`
except the container is never started.  You can then use the 
`docker start <container_id>` command to start the container at any point.

This is useful when you want to set up a container configuration ahead of time
so that it is ready to start when you need it. The initial status of the
new container is `created`.

Please see the [run command](#run) section and the [Docker run reference](
/reference/run/) for more details.

#### Examples

    $ docker create -t -i fedora bash
    6d8af538ec541dd581ebc2a24153a28329acb5268abe5ef868c1f1a261221752
    $ docker start -a -i 6d8af538ec5
    bash-4.2#

As of v1.4.0 container volumes are initialized during the `docker create` phase
(i.e., `docker run` too). For example, this allows you to `create` the `data`
volume container, and then use it from another container:

    $ docker create -v /data --name data ubuntu
    240633dfbb98128fa77473d3d9018f6123b99c454b3251427ae190a7d951ad57
    $ docker run --rm --volumes-from data ubuntu ls -la /data
    total 8
    drwxr-xr-x  2 root root 4096 Dec  5 04:10 .
    drwxr-xr-x 48 root root 4096 Dec  5 04:11 ..

Similarly, `create` a host directory bind mounted volume container, which can
then be used from the subsequent container:

    $ docker create -v /home/docker:/docker --name docker ubuntu
    9aa88c08f319cd1e4515c3c46b0de7cc9aa75e878357b1e96f91e2c773029f03
    $ docker run --rm --volumes-from docker ubuntu ls -la /docker
    total 20
    drwxr-sr-x  5 1000 staff  180 Dec  5 04:00 .
    drwxr-xr-x 48 root root  4096 Dec  5 04:13 ..
    -rw-rw-r--  1 1000 staff 3833 Dec  5 04:01 .ash_history
    -rw-r--r--  1 1000 staff  446 Nov 28 11:51 .ashrc
    -rw-r--r--  1 1000 staff   25 Dec  5 04:00 .gitconfig
    drwxr-sr-x  3 1000 staff   60 Dec  1 03:28 .local
    -rw-r--r--  1 1000 staff  920 Nov 28 11:51 .profile
    drwx--S---  2 1000 staff  460 Dec  5 00:51 .ssh
    drwxr-xr-x 32 1000 staff 1140 Dec  5 04:01 docker


## diff

List the changed files and directories in a container᾿s filesystem

    Usage: docker diff CONTAINER

    Inspect changes on a container's filesystem

There are 3 events that are listed in the `diff`:

1. `A` - Add
2. `D` - Delete
3. `C` - Change

For example:

    $ docker diff 7bb0e258aefe

    C /dev
    A /dev/kmsg
    C /etc
    A /etc/mtab
    A /go
    A /go/src
    A /go/src/github.com
    A /go/src/github.com/docker
    A /go/src/github.com/docker/docker
    A /go/src/github.com/docker/docker/.git
    ....

## events

    Usage: docker events [OPTIONS]

    Get real time events from the server

      -f, --filter=[]    Filter output based on conditions provided
      --since=""         Show all events created since timestamp
      --until=""         Stream events until this timestamp

Docker containers will report the following events:

    create, destroy, die, export, kill, oom, pause, restart, start, stop, unpause

and Docker images will report:

    untag, delete

The `--since` and `--until` parameters can be Unix timestamps, RFC3339
dates or Go duration strings (e.g. `10m`, `1h30m`) computed relative to
client machine’s time. If you do not provide the --since option, the command
returns only new and/or live events.

#### Filtering

The filtering flag (`-f` or `--filter`) format is of "key=value". If you would
like to use multiple filters, pass multiple flags (e.g., 
`--filter "foo=bar" --filter "bif=baz"`)

Using the same filter multiple times will be handled as a *OR*; for example
`--filter container=588a23dac085 --filter container=a8f7720b8c22` will display
events for container 588a23dac085 *OR* container a8f7720b8c22

Using multiple filters will be handled as a *AND*; for example
`--filter container=588a23dac085 --filter event=start` will display events for
container container 588a23dac085 *AND* the event type is *start*

The currently supported filters are:

* container
* event
* image

#### Examples

You'll need two shells for this example.

**Shell 1: Listening for events:**

    $ docker events

**Shell 2: Start and Stop containers:**

    $ docker start 4386fb97867d
    $ docker stop 4386fb97867d
    $ docker stop 7805c1d35632

**Shell 1: (Again .. now showing events):**

    2014-05-10T17:42:14.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) start
    2014-05-10T17:42:14.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) die
    2014-05-10T17:42:14.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) stop
    2014-05-10T17:42:14.999999999Z07:00 7805c1d35632: (from redis:2.8) die
    2014-05-10T17:42:14.999999999Z07:00 7805c1d35632: (from redis:2.8) stop

**Show events in the past from a specified time:**

    $ docker events --since 1378216169
    2014-03-10T17:42:14.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) die
    2014-05-10T17:42:14.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) stop
    2014-05-10T17:42:14.999999999Z07:00 7805c1d35632: (from redis:2.8) die
    2014-03-10T17:42:14.999999999Z07:00 7805c1d35632: (from redis:2.8) stop

    $ docker events --since '2013-09-03'
    2014-09-03T17:42:14.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) start
    2014-09-03T17:42:14.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) die
    2014-05-10T17:42:14.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) stop
    2014-05-10T17:42:14.999999999Z07:00 7805c1d35632: (from redis:2.8) die
    2014-09-03T17:42:14.999999999Z07:00 7805c1d35632: (from redis:2.8) stop

    $ docker events --since '2013-09-03T15:49:29'
    2014-09-03T15:49:29.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) die
    2014-05-10T17:42:14.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) stop
    2014-05-10T17:42:14.999999999Z07:00 7805c1d35632: (from redis:2.8) die
    2014-09-03T15:49:29.999999999Z07:00 7805c1d35632: (from redis:2.8) stop

This example outputs all events that were generated in the last 3 minutes,
relative to the current time on the client machine:

    $ docker events --since '3m'
    2015-05-12T11:51:30.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) die
    2015-05-12T15:52:12.999999999Z07:00 4 4386fb97867d: (from ubuntu-1:14.04) stop
    2015-05-12T15:53:45.999999999Z07:00  7805c1d35632: (from redis:2.8) die
    2015-05-12T15:54:03.999999999Z07:00  7805c1d35632: (from redis:2.8) stop

**Filter events:**

    $ docker events --filter 'event=stop'
    2014-05-10T17:42:14.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) stop
    2014-09-03T17:42:14.999999999Z07:00 7805c1d35632: (from redis:2.8) stop

    $ docker events --filter 'image=ubuntu-1:14.04'
    2014-05-10T17:42:14.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) start
    2014-05-10T17:42:14.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) die
    2014-05-10T17:42:14.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) stop

    $ docker events --filter 'container=7805c1d35632'
    2014-05-10T17:42:14.999999999Z07:00 7805c1d35632: (from redis:2.8) die
    2014-09-03T15:49:29.999999999Z07:00 7805c1d35632: (from redis:2.8) stop

    $ docker events --filter 'container=7805c1d35632' --filter 'container=4386fb97867d'
    2014-09-03T15:49:29.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) die
    2014-05-10T17:42:14.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) stop
    2014-05-10T17:42:14.999999999Z07:00 7805c1d35632: (from redis:2.8) die
    2014-09-03T15:49:29.999999999Z07:00 7805c1d35632: (from redis:2.8) stop

    $ docker events --filter 'container=7805c1d35632' --filter 'event=stop'
    2014-09-03T15:49:29.999999999Z07:00 7805c1d35632: (from redis:2.8) stop

    $ docker events --filter 'container=container_1' --filter 'container=container_2'
    2014-09-03T15:49:29.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) die
    2014-05-10T17:42:14.999999999Z07:00 4386fb97867d: (from ubuntu-1:14.04) stop
    2014-05-10T17:42:14.999999999Z07:00 7805c1d35632: (from redis:2.8) die
    2014-09-03T15:49:29.999999999Z07:00 7805c1d35632: (from redis:2.8) stop

## exec

    Usage: docker exec [OPTIONS] CONTAINER COMMAND [ARG...]

    Run a command in a running container

      -d, --detach=false         Detached mode: run command in the background
      -i, --interactive=false    Keep STDIN open even if not attached
      -t, --tty=false            Allocate a pseudo-TTY
      -u, --user=                Username or UID (format: <name|uid>[:<group|gid>])

The `docker exec` command runs a new command in a running container.

The command started using `docker exec` only runs while the container's primary
process (`PID 1`) is running, and it is not restarted if the container is
restarted.

If the container is paused, then the `docker exec` command will fail with an error:

    $ docker pause test
    test
    $ docker ps
    CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS                   PORTS               NAMES
    1ae3b36715d2        ubuntu:latest       "bash"              17 seconds ago      Up 16 seconds (Paused)                       test
    $ docker exec test ls
    FATA[0000] Error response from daemon: Container test is paused, unpause the container before exec
    $ echo $?
    1

#### Examples

    $ docker run --name ubuntu_bash --rm -i -t ubuntu bash

This will create a container named `ubuntu_bash` and start a Bash session.

    $ docker exec -d ubuntu_bash touch /tmp/execWorks

This will create a new file `/tmp/execWorks` inside the running container
`ubuntu_bash`, in the background.

    $ docker exec -it ubuntu_bash bash

This will create a new Bash session in the container `ubuntu_bash`.

## export

    Usage: docker export [OPTIONS] CONTAINER

    Export the contents of a filesystem to a tar archive (streamed to STDOUT by default)

      -o, --output=""    Write to a file, instead of STDOUT

      Produces a tarred repository to the standard output stream.

   For example:

    $ docker export red_panda > latest.tar

   Or

    $ docker export --output="latest.tar" red_panda

> **Note:**
> `docker export` does not export the contents of volumes associated
> with the container. If a volume is mounted on top of an existing
> directory in the container, `docker export` will export the contents
> of the *underlying* directory, not the contents of the volume.
>
> Refer to [Backup, restore, or migrate data volumes](/userguide/dockervolumes/#backup-restore-or-migrate-data-volumes)
> in the user guide for examples on exporting data in a volume.

## history

    Usage: docker history [OPTIONS] IMAGE

    Show the history of an image

      -H, --human=true     Print sizes and dates in human readable format
      --no-trunc=false     Don't truncate output
      -q, --quiet=false    Only show numeric IDs

To see how the `docker:latest` image was built:

    $ docker history docker
    IMAGE               CREATED             CREATED BY                                      SIZE                COMMENT
    3e23a5875458        8 days ago          /bin/sh -c #(nop) ENV LC_ALL=C.UTF-8            0 B
    8578938dd170        8 days ago          /bin/sh -c dpkg-reconfigure locales &&    loc   1.245 MB
    be51b77efb42        8 days ago          /bin/sh -c apt-get update && apt-get install    338.3 MB
    4b137612be55        6 weeks ago         /bin/sh -c #(nop) ADD jessie.tar.xz in /        121 MB
    750d58736b4b        6 weeks ago         /bin/sh -c #(nop) MAINTAINER Tianon Gravi <ad   0 B
    511136ea3c5a        9 months ago                                                        0 B                 Imported from -

To see how the `docker:apache` image was added to a container's base image:

    $ docker history docker:scm
    IMAGE               CREATED             CREATED BY                                      SIZE                COMMENT
    2ac9d1098bf1        3 months ago        /bin/bash                                       241.4 MB            Added Apache to Fedora base image
    88b42ffd1f7c        5 months ago        /bin/sh -c #(nop) ADD file:1fd8d7f9f6557cafc7   373.7 MB
    c69cab00d6ef        5 months ago        /bin/sh -c #(nop) MAINTAINER Lokesh Mandvekar   0 B
    511136ea3c5a        19 months ago                                                       0 B                 Imported from -


## images

    Usage: docker images [OPTIONS] [REPOSITORY]

    List images

      -a, --all=false      Show all images (default hides intermediate images)
      --digests=false      Show digests
      -f, --filter=[]      Filter output based on conditions provided
      --help=false         Print usage
      --no-trunc=false     Don't truncate output
      -q, --quiet=false    Only show numeric IDs

The default `docker images` will show all top level
images, their repository and tags, and their virtual size.

Docker images have intermediate layers that increase reusability,
decrease disk usage, and speed up `docker build` by
allowing each step to be cached. These intermediate layers are not shown
by default.

The `VIRTUAL SIZE` is the cumulative space taken up by the image and all
its parent images. This is also the disk space used by the contents of the
Tar file created when you `docker save` an image.

An image will be listed more than once if it has multiple repository names
or tags. This single image (identifiable by its matching `IMAGE ID`)
uses up the `VIRTUAL SIZE` listed only once.

#### Listing the most recently created images

    $ docker images
    REPOSITORY                TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
    <none>                    <none>              77af4d6b9913        19 hours ago        1.089 GB
    committ                   latest              b6fa739cedf5        19 hours ago        1.089 GB
    <none>                    <none>              78a85c484f71        19 hours ago        1.089 GB
    docker                    latest              30557a29d5ab        20 hours ago        1.089 GB
    <none>                    <none>              5ed6274db6ce        24 hours ago        1.089 GB
    postgres                  9                   746b819f315e        4 days ago          213.4 MB
    postgres                  9.3                 746b819f315e        4 days ago          213.4 MB
    postgres                  9.3.5               746b819f315e        4 days ago          213.4 MB
    postgres                  latest              746b819f315e        4 days ago          213.4 MB


#### Listing the full length image IDs

    $ docker images --no-trunc
    REPOSITORY                    TAG                 IMAGE ID                                                           CREATED             VIRTUAL SIZE
    <none>                        <none>              77af4d6b9913e693e8d0b4b294fa62ade6054e6b2f1ffb617ac955dd63fb0182   19 hours ago        1.089 GB
    committest                    latest              b6fa739cedf5ea12a620a439402b6004d057da800f91c7524b5086a5e4749c9f   19 hours ago        1.089 GB
    <none>                        <none>              78a85c484f71509adeaace20e72e941f6bdd2b25b4c75da8693efd9f61a37921   19 hours ago        1.089 GB
    docker                        latest              30557a29d5abc51e5f1d5b472e79b7e296f595abcf19fe6b9199dbbc809c6ff4   20 hours ago        1.089 GB
    <none>                        <none>              0124422dd9f9cf7ef15c0617cda3931ee68346455441d66ab8bdc5b05e9fdce5   20 hours ago        1.089 GB
    <none>                        <none>              18ad6fad340262ac2a636efd98a6d1f0ea775ae3d45240d3418466495a19a81b   22 hours ago        1.082 GB
    <none>                        <none>              f9f1e26352f0a3ba6a0ff68167559f64f3e21ff7ada60366e2d44a04befd1d3a   23 hours ago        1.089 GB
    tryout                        latest              2629d1fa0b81b222fca63371ca16cbf6a0772d07759ff80e8d1369b926940074   23 hours ago        131.5 MB
    <none>                        <none>              5ed6274db6ceb2397844896966ea239290555e74ef307030ebb01ff91b1914df   24 hours ago        1.089 GB

#### Listing image digests

Images that use the v2 or later format have a content-addressable identifier
called a `digest`. As long as the input used to generate the image is
unchanged, the digest value is predictable. To list image digest values, use
the `--digests` flag:

    $ docker images --digests
    REPOSITORY                         TAG                 DIGEST                                                                    IMAGE ID            CREATED             VIRTUAL SIZE
    localhost:5000/test/busybox        <none>              sha256:cbbf2f9a99b47fc460d422812b6a5adff7dfee951d8fa2e4a98caa0382cfbdbf   4986bf8c1536        9 weeks ago         2.43 MB

When pushing or pulling to a 2.0 registry, the `push` or `pull` command
output includes the image digest. You can `pull` using a digest value. You can
also reference by digest in `create`, `run`, and `rmi` commands, as well as the
`FROM` image reference in a Dockerfile.

#### Filtering

The filtering flag (`-f` or `--filter`) format is of "key=value". If there is more
than one filter, then pass multiple flags (e.g., `--filter "foo=bar" --filter "bif=baz"`)

The currently supported filters are:

* dangling (boolean - true or false)
* label (`label=<key>` or `label=<key>=<value>`)

##### Untagged images

    $ docker images --filter "dangling=true"

    REPOSITORY          TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
    <none>              <none>              8abc22fbb042        4 weeks ago         0 B
    <none>              <none>              48e5f45168b9        4 weeks ago         2.489 MB
    <none>              <none>              bf747efa0e2f        4 weeks ago         0 B
    <none>              <none>              980fe10e5736        12 weeks ago        101.4 MB
    <none>              <none>              dea752e4e117        12 weeks ago        101.4 MB
    <none>              <none>              511136ea3c5a        8 months ago        0 B

This will display untagged images, that are the leaves of the images tree (not
intermediary layers). These images occur when a new build of an image takes the
`repo:tag` away from the image ID, leaving it untagged. A warning will be issued
if trying to remove an image when a container is presently using it.
By having this flag it allows for batch cleanup.

Ready for use by `docker rmi ...`, like:

    $ docker rmi $(docker images -f "dangling=true" -q)

    8abc22fbb042
    48e5f45168b9
    bf747efa0e2f
    980fe10e5736
    dea752e4e117
    511136ea3c5a

NOTE: Docker will warn you if any containers exist that are using these untagged images.

## import

    Usage: docker import URL|- [REPOSITORY[:TAG]]

    Create an empty filesystem image and import the contents of the
	tarball (.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz) into it, then
	optionally tag it.

      -c, --change=[]     Apply specified Dockerfile instructions while importing the image

URLs must start with `http` and point to a single file archive (.tar,
.tar.gz, .tgz, .bzip, .tar.xz, or .txz) containing a root filesystem. If
you would like to import from a local directory or archive, you can use
the `-` parameter to take the data from `STDIN`.

The `--change` option will apply `Dockerfile` instructions to the image
that is created.
Supported `Dockerfile` instructions:
`CMD`|`ENTRYPOINT`|`ENV`|`EXPOSE`|`ONBUILD`|`USER`|`VOLUME`|`WORKDIR`

#### Examples

**Import from a remote location:**

This will create a new untagged image.

    $ docker import http://example.com/exampleimage.tgz

**Import from a local file:**

Import to docker via pipe and `STDIN`.

    $ cat exampleimage.tgz | docker import - exampleimagelocal:new

**Import from a local directory:**

    $ sudo tar -c . | docker import - exampleimagedir

**Import from a local directory with new configurations:**

    $ sudo tar -c . | docker import --change "ENV DEBUG true" - exampleimagedir

Note the `sudo` in this example – you must preserve
the ownership of the files (especially root ownership) during the
archiving with tar. If you are not root (or the sudo command) when you
tar, then the ownerships might not get preserved.

## info


    Usage: docker info

    Display system-wide information

For example:

    $ docker -D info
    Containers: 14
    Images: 52
    Storage Driver: aufs
     Root Dir: /var/lib/docker/aufs
     Backing Filesystem: extfs
     Dirs: 545
    Execution Driver: native-0.2
    Logging Driver: json-file
    Kernel Version: 3.13.0-24-generic
    Operating System: Ubuntu 14.04 LTS
    CPUs: 1
    Name: prod-server-42
    ID: 7TRN:IPZB:QYBB:VPBQ:UMPP:KARE:6ZNR:XE6T:7EWV:PKF4:ZOJD:TPYS
    Total Memory: 2 GiB
    Debug mode (server): false
    Debug mode (client): true
    File Descriptors: 10
    Goroutines: 9
    System Time: Tue Mar 10 18:38:57 UTC 2015
    EventsListeners: 0
    Init Path: /usr/bin/docker
    Docker Root Dir: /var/lib/docker
    Http Proxy: http://test:test@localhost:8080
    Https Proxy: https://test:test@localhost:8080
    No Proxy: 9.81.1.160
    Username: svendowideit
    Registry: [https://index.docker.io/v1/]
    Labels:
     storage=ssd

The global `-D` option tells all `docker` commands to output debug information.

When sending issue reports, please use `docker version` and `docker -D info` to
ensure we know how your setup is configured.

## inspect

    Usage: docker inspect [OPTIONS] CONTAINER|IMAGE [CONTAINER|IMAGE...]

    Return low-level information on a container or image

      -f, --format=""    Format the output using the given go template

By default, this will render all results in a JSON array. If a format is
specified, the given template will be executed for each result.

Go's [text/template](http://golang.org/pkg/text/template/) package
describes all the details of the format.

#### Examples

**Get an instance's IP address:**

For the most part, you can pick out any field from the JSON in a fairly
straightforward manner.

    $ docker inspect --format='{{.NetworkSettings.IPAddress}}' $INSTANCE_ID

**Get an instance's MAC Address:**

For the most part, you can pick out any field from the JSON in a fairly
straightforward manner.

    $ docker inspect --format='{{.NetworkSettings.MacAddress}}' $INSTANCE_ID

**Get an instance's log path:**

    $ docker inspect --format='{{.LogPath}}' $INSTANCE_ID

**List All Port Bindings:**

One can loop over arrays and maps in the results to produce simple text
output:

    $ docker inspect --format='{{range $p, $conf := .NetworkSettings.Ports}} {{$p}} -> {{(index $conf 0).HostPort}} {{end}}' $INSTANCE_ID

**Find a Specific Port Mapping:**

The `.Field` syntax doesn't work when the field name begins with a
number, but the template language's `index` function does. The
`.NetworkSettings.Ports` section contains a map of the internal port
mappings to a list of external address/port objects, so to grab just the
numeric public port, you use `index` to find the specific port map, and
then `index` 0 contains the first object inside of that. Then we ask for
the `HostPort` field to get the public address.

    $ docker inspect --format='{{(index (index .NetworkSettings.Ports "8787/tcp") 0).HostPort}}' $INSTANCE_ID

**Get config:**

The `.Field` syntax doesn't work when the field contains JSON data, but
the template language's custom `json` function does. The `.config`
section contains complex JSON object, so to grab it as JSON, you use
`json` to convert the configuration object into JSON.

    $ docker inspect --format='{{json .config}}' $INSTANCE_ID

## kill

    Usage: docker kill [OPTIONS] CONTAINER [CONTAINER...]

    Kill a running container using SIGKILL or a specified signal

      -s, --signal="KILL"    Signal to send to the container

The main process inside the container will be sent `SIGKILL`, or any
signal specified with option `--signal`.

## load

    Usage: docker load [OPTIONS]

    Load an image from a tar archive on STDIN

      -i, --input=""     Read from a tar archive file, instead of STDIN

Loads a tarred repository from a file or the standard input stream.
Restores both images and tags.

    $ docker images
    REPOSITORY          TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
    $ docker load < busybox.tar
    $ docker images
    REPOSITORY          TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
    busybox             latest              769b9341d937        7 weeks ago         2.489 MB
    $ docker load --input fedora.tar
    $ docker images
    REPOSITORY          TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
    busybox             latest              769b9341d937        7 weeks ago         2.489 MB
    fedora              rawhide             0d20aec6529d        7 weeks ago         387 MB
    fedora              20                  58394af37342        7 weeks ago         385.5 MB
    fedora              heisenbug           58394af37342        7 weeks ago         385.5 MB
    fedora              latest              58394af37342        7 weeks ago         385.5 MB

## login

    Usage: docker login [OPTIONS] [SERVER]

    Register or log in to a Docker registry server, if no server is
	specified "https://index.docker.io/v1/" is the default.

      -e, --email=""       Email
      -p, --password=""    Password
      -u, --username=""    Username

If you want to login to a self-hosted registry you can specify this by
adding the server name.

    example:
    $ docker login localhost:8080

## logout

    Usage: docker logout [SERVER]

    Log out from a Docker registry, if no server is
	specified "https://index.docker.io/v1/" is the default.

For example:

    $ docker logout localhost:8080

## logs

    Usage: docker logs [OPTIONS] CONTAINER

    Fetch the logs of a container

      -f, --follow=false        Follow log output
      --since=""                Show logs since timestamp
      -t, --timestamps=false    Show timestamps
      --tail="all"              Number of lines to show from the end of the logs

NOTE: this command is available only for containers with `json-file` logging
driver.

The `docker logs` command batch-retrieves logs present at the time of execution.

The `docker logs --follow` command will continue streaming the new output from
the container's `STDOUT` and `STDERR`.

Passing a negative number or a non-integer to `--tail` is invalid and the
value is set to `all` in that case. This behavior may change in the future.

The `docker logs --timestamp` commands will add an RFC3339Nano
timestamp, for example `2014-09-16T06:17:46.000000000Z`, to each
log entry. To ensure that the timestamps for are aligned the
nano-second part of the timestamp will be padded with zero when necessary.

The `--since` option shows only the container logs generated after
a given date. You can specify the date as an RFC 3339 date, a UNIX
timestamp, or a Go duration string (e.g. `1m30s`, `3h`). Docker computes
the date relative to the client machine’s time. You can combine
the `--since` option with either or both of the `--follow` or `--tail` options.

## pause

    Usage: docker pause CONTAINER [CONTAINER...]

    Pause all processes within a container

The `docker pause` command uses the cgroups freezer to suspend all processes in
a container. Traditionally, when suspending a process the `SIGSTOP` signal is
used, which is observable by the process being suspended. With the cgroups freezer
the process is unaware, and unable to capture, that it is being suspended,
and subsequently resumed.

See the
[cgroups freezer documentation](https://www.kernel.org/doc/Documentation/cgroups/freezer-subsystem.txt)
for further details.

## port

    Usage: docker port CONTAINER [PRIVATE_PORT[/PROTO]]

    List port mappings for the CONTAINER, or lookup the public-facing port that is
	NAT-ed to the PRIVATE_PORT

You can find out all the ports mapped by not specifying a `PRIVATE_PORT`, or
just a specific mapping:

    $ docker ps test
    CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS                                            NAMES
    b650456536c7        busybox:latest      top                 54 minutes ago      Up 54 minutes       0.0.0.0:1234->9876/tcp, 0.0.0.0:4321->7890/tcp   test
    $ docker port test
    7890/tcp -> 0.0.0.0:4321
    9876/tcp -> 0.0.0.0:1234
    $ docker port test 7890/tcp
    0.0.0.0:4321
    $ docker port test 7890/udp
    2014/06/24 11:53:36 Error: No public port '7890/udp' published for test
    $ docker port test 7890
    0.0.0.0:4321

## ps

    Usage: docker ps [OPTIONS]

    List containers

      -a, --all=false       Show all containers (default shows just running)
      --before=""           Show only container created before Id or Name
      -f, --filter=[]       Filter output based on conditions provided
      -l, --latest=false    Show the latest created container, include non-running
      -n=-1                 Show n last created containers, include non-running
      --no-trunc=false      Don't truncate output
      -q, --quiet=false     Only display numeric IDs
      -s, --size=false      Display total file sizes
      --since=""            Show created since Id or Name, include non-running

Running `docker ps --no-trunc` showing 2 linked containers.

    $ docker ps
    CONTAINER ID        IMAGE                        COMMAND                CREATED              STATUS              PORTS               NAMES
    4c01db0b339c        ubuntu:12.04                 bash                   17 seconds ago       Up 16 seconds       3300-3310/tcp       webapp
    d7886598dbe2        crosbymichael/redis:latest   /redis-server --dir    33 minutes ago       Up 33 minutes       6379/tcp            redis,webapp/db

`docker ps` will show only running containers by default. To see all containers:
`docker ps -a`

`docker ps` will group exposed ports into a single range if possible. E.g., a container that exposes TCP ports `100, 101, 102` will display `100-102/tcp` in the `PORTS` column.

#### Filtering

The filtering flag (`-f` or `--filter)` format is a `key=value` pair. If there is more
than one filter, then pass multiple flags (e.g. `--filter "foo=bar" --filter "bif=baz"`)

The currently supported filters are:

* id (container's id)
* label (`label=<key>` or `label=<key>=<value>`)
* name (container's name)
* exited (int - the code of exited containers. Only useful with `--all`)
* status (created|restarting|running|paused|exited)

##### Successfully exited containers

    $ docker ps -a --filter 'exited=0'
    CONTAINER ID        IMAGE             COMMAND                CREATED             STATUS                   PORTS                      NAMES
    ea09c3c82f6e        registry:latest   /srv/run.sh            2 weeks ago         Exited (0) 2 weeks ago   127.0.0.1:5000->5000/tcp   desperate_leakey
    106ea823fe4e        fedora:latest     /bin/sh -c 'bash -l'   2 weeks ago         Exited (0) 2 weeks ago                              determined_albattani
    48ee228c9464        fedora:20         bash                   2 weeks ago         Exited (0) 2 weeks ago                              tender_torvalds

This shows all the containers that have exited with status of '0'

## pull

    Usage: docker pull [OPTIONS] NAME[:TAG] | [REGISTRY_HOST[:REGISTRY_PORT]/]NAME[:TAG]

    Pull an image or a repository from the registry

      -a, --all-tags=false    Download all tagged images in the repository

Most of your images will be created on top of a base image from the
[Docker Hub](https://hub.docker.com) registry.

[Docker Hub](https://hub.docker.com) contains many pre-built images that you
can `pull` and try without needing to define and configure your own.

It is also possible to manually specify the path of a registry to pull from.
For example, if you have set up a local registry, you can specify its path to
pull from it. A repository path is similar to a URL, but does not contain
a protocol specifier (`https://`, for example).

To download a particular image, or set of images (i.e., a repository),
use `docker pull`:

    $ docker pull debian
    # will pull the debian:latest image and its intermediate layers
    $ docker pull debian:testing
    # will pull the image named debian:testing and any intermediate
    # layers it is based on.
    $ docker pull debian@sha256:cbbf2f9a99b47fc460d422812b6a5adff7dfee951d8fa2e4a98caa0382cfbdbf
    # will pull the image from the debian repository with the digest
    # sha256:cbbf2f9a99b47fc460d422812b6a5adff7dfee951d8fa2e4a98caa0382cfbdbf
    # and any intermediate layers it is based on.
    # (Typically the empty `scratch` image, a MAINTAINER layer,
    # and the un-tarred base).
    $ docker pull --all-tags centos
    # will pull all the images from the centos repository
    $ docker pull registry.hub.docker.com/debian
    # manually specifies the path to the default Docker registry. This could
    # be replaced with the path to a local registry to pull from another source.
    # sudo docker pull myhub.com:8080/test-image

## push

    Usage: docker push NAME[:TAG]

    Push an image or a repository to the registry

Use `docker push` to share your images to the [Docker Hub](https://hub.docker.com)
registry or to a self-hosted one.

## rename

    Usage: docker rename OLD_NAME NEW_NAME

    rename a existing container to a NEW_NAME

The `docker rename` command allows the container to be renamed to a different name.

## restart

    Usage: docker restart [OPTIONS] CONTAINER [CONTAINER...]

    Restart a running container

      -t, --time=10      Seconds to wait for stop before killing the container

## rm

    Usage: docker rm [OPTIONS] CONTAINER [CONTAINER...]

    Remove one or more containers

      -f, --force=false      Force the removal of a running container (uses SIGKILL)
      -l, --link=false       Remove the specified link
      -v, --volumes=false    Remove the volumes associated with the container

#### Examples

    $ docker rm /redis
    /redis

This will remove the container referenced under the link
`/redis`.

    $ docker rm --link /webapp/redis
    /webapp/redis

This will remove the underlying link between `/webapp` and the `/redis`
containers removing all network communication.

    $ docker rm --force redis
    redis

The main process inside the container referenced under the link `/redis` will receive
`SIGKILL`, then the container will be removed.

    $ docker rm $(docker ps -a -q)

This command will delete all stopped containers. The command 
`docker ps -a -q` will return all existing container IDs and pass them to 
the `rm` command which will delete them. Any running containers will not be
deleted.

## rmi

    Usage: docker rmi [OPTIONS] IMAGE [IMAGE...]

    Remove one or more images

      -f, --force=false    Force removal of the image
      --no-prune=false     Do not delete untagged parents

#### Removing tagged images

You can remove an image using its short or long ID, its tag, or its digest. If
an image has one or more tag or digest reference, you must remove all of them
before the image is removed.

    $ docker images
    REPOSITORY                TAG                 IMAGE ID            CREATED             SIZE
    test1                     latest              fd484f19954f        23 seconds ago      7 B (virtual 4.964 MB)
    test                      latest              fd484f19954f        23 seconds ago      7 B (virtual 4.964 MB)
    test2                     latest              fd484f19954f        23 seconds ago      7 B (virtual 4.964 MB)

    $ docker rmi fd484f19954f
    Error: Conflict, cannot delete image fd484f19954f because it is tagged in multiple repositories, use -f to force
    2013/12/11 05:47:16 Error: failed to remove one or more images

    $ docker rmi test1
    Untagged: test1:latest
    $ docker rmi test2
    Untagged: test2:latest

    $ docker images
    REPOSITORY                TAG                 IMAGE ID            CREATED             SIZE
    test                      latest              fd484f19954f        23 seconds ago      7 B (virtual 4.964 MB)
    $ docker rmi test
    Untagged: test:latest
    Deleted: fd484f19954f4920da7ff372b5067f5b7ddb2fd3830cecd17b96ea9e286ba5b8

If you use the `-f` flag and specify the image's short or long ID, then this
command untags and removes all images that match the specified ID.

    $ docker images
    REPOSITORY                TAG                 IMAGE ID            CREATED             SIZE
    test1                     latest              fd484f19954f        23 seconds ago      7 B (virtual 4.964 MB)
    test                      latest              fd484f19954f        23 seconds ago      7 B (virtual 4.964 MB)
    test2                     latest              fd484f19954f        23 seconds ago      7 B (virtual 4.964 MB)

    $ docker rmi -f fd484f19954f
    Untagged: test1:latest
    Untagged: test:latest
    Untagged: test2:latest
    Deleted: fd484f19954f4920da7ff372b5067f5b7ddb2fd3830cecd17b96ea9e286ba5b8

An image pulled by digest has no tag associated with it:

    $ docker images --digests
    REPOSITORY                     TAG       DIGEST                                                                    IMAGE ID        CREATED         VIRTUAL SIZE
    localhost:5000/test/busybox    <none>    sha256:cbbf2f9a99b47fc460d422812b6a5adff7dfee951d8fa2e4a98caa0382cfbdbf   4986bf8c1536    9 weeks ago     2.43 MB

To remove an image using its digest:

    $ docker rmi localhost:5000/test/busybox@sha256:cbbf2f9a99b47fc460d422812b6a5adff7dfee951d8fa2e4a98caa0382cfbdbf
    Untagged: localhost:5000/test/busybox@sha256:cbbf2f9a99b47fc460d422812b6a5adff7dfee951d8fa2e4a98caa0382cfbdbf
    Deleted: 4986bf8c15363d1c5d15512d5266f8777bfba4974ac56e3270e7760f6f0a8125
    Deleted: ea13149945cb6b1e746bf28032f02e9b5a793523481a0a18645fc77ad53c4ea2
    Deleted: df7546f9f060a2268024c8a230d8639878585defcc1bc6f79d2728a13957871b

## run

    Usage: docker run [OPTIONS] IMAGE [COMMAND] [ARG...]

    Run a command in a new container

      -a, --attach=[]            Attach to STDIN, STDOUT or STDERR
      --add-host=[]              Add a custom host-to-IP mapping (host:ip)
      --blkio-weight=0           Block IO weight (relative weight)
      -c, --cpu-shares=0         CPU shares (relative weight)
      --cap-add=[]               Add Linux capabilities
      --cap-drop=[]              Drop Linux capabilities
      --cidfile=""               Write the container ID to the file
      --cpuset-cpus=""           CPUs in which to allow execution (0-3, 0,1)
      --cpuset-mems=""           Memory nodes (MEMs) in which to allow execution (0-3, 0,1)
      --cpu-period=0             Limit the CPU CFS (Completely Fair Scheduler) period
      --cpu-quota=0              Limit the CPU CFS (Completely Fair Scheduler) quota
      -d, --detach=false         Run container in background and print container ID
      --device=[]                Add a host device to the container
      --dns=[]                   Set custom DNS servers
      --dns-search=[]            Set custom DNS search domains
      -e, --env=[]               Set environment variables
      --entrypoint=""            Overwrite the default ENTRYPOINT of the image
      --env-file=[]              Read in a file of environment variables
      --expose=[]                Expose a port or a range of ports
      -h, --hostname=""          Container host name
      --help=false               Print usage
      -i, --interactive=false    Keep STDIN open even if not attached
      --ipc=""                   IPC namespace to use
      --link=[]                  Add link to another container
      --log-driver=""            Logging driver for container
      --log-opt=[]               Log driver specific options
      --lxc-conf=[]              Add custom lxc options
      -m, --memory=""            Memory limit
      -l, --label=[]             Set metadata on the container (e.g., --label=com.example.key=value)
      --label-file=[]            Read in a file of labels (EOL delimited)
      --mac-address=""           Container MAC address (e.g. 92:d0:c6:0a:29:33)
      --memory-swap=""           Total memory (memory + swap), '-1' to disable swap
      --name=""                  Assign a name to the container
      --net="bridge"             Set the Network mode for the container
      --oom-kill-disable=false   Whether to disable OOM Killer for the container or not
      -P, --publish-all=false    Publish all exposed ports to random ports
      -p, --publish=[]           Publish a container's port(s) to the host
      --pid=""                   PID namespace to use
      --uts=""                   UTS namespace to use
      --privileged=false         Give extended privileges to this container
      --read-only=false          Mount the container's root filesystem as read only
      --restart="no"             Restart policy (no, on-failure[:max-retry], always)
      --rm=false                 Automatically remove the container when it exits
      --security-opt=[]          Security Options
      --sig-proxy=true           Proxy received signals to the process
      -t, --tty=false            Allocate a pseudo-TTY
      -u, --user=""              Username or UID (format: <name|uid>[:<group|gid>])
      -v, --volume=[]            Bind mount a volume
      --volumes-from=[]          Mount volumes from the specified container(s)
      -w, --workdir=""           Working directory inside the container

The `docker run` command first `creates` a writeable container layer over the
specified image, and then `starts` it using the specified command. That is,
`docker run` is equivalent to the API `/containers/create` then
`/containers/(id)/start`. A stopped container can be restarted with all its
previous changes intact using `docker start`. See `docker ps -a` to view a list
of all containers.

There is detailed information about `docker run` in the [Docker run reference](
/reference/run/).

The `docker run` command can be used in combination with `docker commit` to
[*change the command that a container runs*](#commit-an-existing-container).

See the [Docker User Guide](/userguide/dockerlinks/) for more detailed
information about the `--expose`, `-p`, `-P` and `--link` parameters,
and linking containers.

#### Examples

    $ docker run --name test -it debian
    $$ exit 13
    exit
    $ echo $?
    13
    $ docker ps -a | grep test
    275c44472aeb        debian:7            "/bin/bash"         26 seconds ago      Exited (13) 17 seconds ago                         test

In this example, we are running `bash` interactively in the `debian:latest` image, and giving
the container the name `test`. We then quit `bash` by running `exit 13`, which means `bash`
will have an exit code of `13`. This is then passed on to the caller of `docker run`, and
is recorded in the `test` container metadata.

    $ docker run --cidfile /tmp/docker_test.cid ubuntu echo "test"

This will create a container and print `test` to the console. The `cidfile`
flag makes Docker attempt to create a new file and write the container ID to it.
If the file exists already, Docker will return an error. Docker will close this
file when `docker run` exits.

    $ docker run -t -i --rm ubuntu bash
    root@bc338942ef20:/# mount -t tmpfs none /mnt
    mount: permission denied

This will *not* work, because by default, most potentially dangerous kernel
capabilities are dropped; including `cap_sys_admin` (which is required to mount
filesystems). However, the `--privileged` flag will allow it to run:

    $ docker run --privileged ubuntu bash
    root@50e3f57e16e6:/# mount -t tmpfs none /mnt
    root@50e3f57e16e6:/# df -h
    Filesystem      Size  Used Avail Use% Mounted on
    none            1.9G     0  1.9G   0% /mnt

The `--privileged` flag gives *all* capabilities to the container, and it also
lifts all the limitations enforced by the `device` cgroup controller. In other
words, the container can then do almost everything that the host can do. This
flag exists to allow special use-cases, like running Docker within Docker.

    $ docker  run -w /path/to/dir/ -i -t  ubuntu pwd

The `-w` lets the command being executed inside directory given, here
`/path/to/dir/`. If the path does not exists it is created inside the container.

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

    $ docker run -t -i -v /var/run/docker.sock:/var/run/docker.sock -v ./static-docker:/usr/bin/docker busybox sh

By bind-mounting the docker unix socket and statically linked docker
binary (such as that provided by [https://get.docker.com](
https://get.docker.com)), you give the container the full access to create and
manipulate the host's Docker daemon.

    $ docker run -p 127.0.0.1:80:8080 ubuntu bash

This binds port `8080` of the container to port `80` on `127.0.0.1` of
the host machine. The [Docker User Guide](/userguide/dockerlinks/)
explains in detail how to manipulate ports in Docker.

    $ docker run --expose 80 ubuntu bash

This exposes port `80` of the container for use within a link without
publishing the port to the host system's interfaces. The [Docker User
Guide](/userguide/dockerlinks) explains in detail how to manipulate
ports in Docker.

    $ docker run -e MYVAR1 --env MYVAR2=foo --env-file ./env.list ubuntu bash

This sets environmental variables in the container. For illustration all three
flags are shown here. Where `-e`, `--env` take an environment variable and
value, or if no `=` is provided, then that variable's current value is passed
through (i.e. `$MYVAR1` from the host is set to `$MYVAR1` in the container).
When no `=` is provided and that variable is not defined in the client's
environment then that variable will be removed from the container's list of
environment variables.
All three flags, `-e`, `--env` and `--env-file` can be repeated.

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

    # pass through this variable from the caller
    TEST_PASSTHROUGH
    $ sudo TEST_PASSTHROUGH=howdy docker run --env-file ./env.list busybox env
    HOME=/
    PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
    HOSTNAME=5198e0745561
    TEST_FOO=BAR
    TEST_APP_DEST_HOST=10.10.0.127
    TEST_APP_DEST_PORT=8888
    TEST_PASSTHROUGH=howdy

    $ docker run --name console -t -i ubuntu bash

A label is a a `key=value` pair that applies metadata to a container. To label a container with two labels:

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
metadata in Docker*](/userguide/labels-custom-metadata/) in the Docker User
Guide.

    $ docker run --link /redis:redis --name console ubuntu bash

The `--link` flag will link the container named `/redis` into the newly
created container with the alias `redis`. The new container can access the
network and environment of the `redis` container via environment variables.
The `--link` flag will also just accept the form `<name or id>` in which case
the alias will match the name. For instance, you could have written the previous
example as:

    $ docker run --link redis --name console ubuntu bash

The `--name` flag will assign the name `console` to the newly created
container.

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
    $ docker run --device=/dev/sda:/dev/xvdc:ro --rm -it ubuntu fdisk  /dev/xvdc
    You will not be able to write the partition table.

    Command (m for help): q

    $ docker run --device=/dev/sda:/dev/xvdc --rm -it ubuntu fdisk  /dev/xvdc

    Command (m for help): q

    $ docker run --device=/dev/sda:/dev/xvdc:m --rm -it ubuntu fdisk  /dev/xvdc
    fdisk: unable to open /dev/xvdc: Operation not permitted

> **Note:**
> `--device` cannot be safely used with ephemeral devices. Block devices
> that may be removed should not be added to untrusted containers with
> `--device`.

**A complete example:**

    $ docker run -d --name static static-web-files sh
    $ docker run -d --expose=8098 --name riak riakserver
    $ docker run -d -m 100m -e DEVELOPMENT=1 -e BRANCH=example-code -v $(pwd):/app/bin:ro --name app appserver
    $ docker run -d -p 1443:443 --dns=10.0.0.1 --dns-search=dev.org -v /var/log/httpd --volumes-from static --link riak --link app -h www.sven.dev.org --name web webserver
    $ docker run -t -i --rm --volumes-from web -w /var/log/httpd busybox tail -f access.log

This example shows five containers that might be set up to test a web
application change:

1. Start a pre-prepared volume image `static-web-files` (in the background)
   that has CSS, image and static HTML in it, (with a `VOLUME` instruction in
   the Dockerfile to allow the web server to use those files);
2. Start a pre-prepared `riakserver` image, give the container name `riak` and
   expose port `8098` to any containers that link to it;
3. Start the `appserver` image, restricting its memory usage to 100MB, setting
   two environment variables `DEVELOPMENT` and `BRANCH` and bind-mounting the
   current directory (`$(pwd)`) in the container in read-only mode as `/app/bin`;
4. Start the `webserver`, mapping port `443` in the container to port `1443` on
   the Docker server, setting the DNS server to `10.0.0.1` and DNS search
   domain to `dev.org`, creating a volume to put the log files into (so we can
   access it from another container), then importing the files from the volume
   exposed by the `static` container, and linking to all exposed ports from
   `riak` and `app`. Lastly, we set the hostname to `web.sven.dev.org` so its
   consistent with the pre-generated SSL certificate;
5. Finally, we create a container that runs `tail -f access.log` using the logs
   volume from the `web` container, setting the workdir to `/var/log/httpd`. The
   `--rm` option means that when the container exits, the container's layer is
   removed.

#### Restart policies

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
        the container indefinitely.
      </td>
    </tr>
  </tbody>
</table>

    $ docker run --restart=always redis

This will run the `redis` container with a restart policy of **always**
so that if the container exits, Docker will restart it.

More detailed information on restart policies can be found in the
[Restart Policies (--restart)](/reference/run/#restart-policies-restart)
section of the Docker run reference page.

### Adding entries to a container hosts file

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

### Setting ulimits in a container

Since setting `ulimit` settings in a container requires extra privileges not
available in the default container, you can set these using the `--ulimit` flag.
`--ulimit` is specified with a soft and hard limit as such:
`<type>=<soft limit>[:<hard limit>]`, for example:

    $ docker run --ulimit nofile=1024:1024 --rm debian ulimit -n
    1024

> **Note:**
> If you do not provide a `hard limit`, the `soft limit` will be used
> for both values. If no `ulimits` are set, they will be inherited from
> the default `ulimits` set on the daemon.  `as` option is disabled now.
> In other words, the following script is not supported:
> `$ docker run -it --ulimit as=1024 fedora /bin/bash`

The values are sent to the appropriate `syscall` as they are set.
Docker doesn't perform any byte conversion. Take this into account when setting the values.

## save

    Usage: docker save [OPTIONS] IMAGE [IMAGE...]

    Save an image(s) to a tar archive (streamed to STDOUT by default)

      -o, --output=""    Write to a file, instead of STDOUT

Produces a tarred repository to the standard output stream.
Contains all parent layers, and all tags + versions, or specified `repo:tag`, for
each argument provided.

It is used to create a backup that can then be used with `docker load`

    $ docker save busybox > busybox.tar
    $ ls -sh busybox.tar
    2.7M busybox.tar
    $ docker save --output busybox.tar busybox
    $ ls -sh busybox.tar
    2.7M busybox.tar
    $ docker save -o fedora-all.tar fedora
    $ docker save -o fedora-latest.tar fedora:latest

It is even useful to cherry-pick particular tags of an image repository

    $ docker save -o ubuntu.tar ubuntu:lucid ubuntu:saucy

## search

Search [Docker Hub](https://hub.docker.com) for images

    Usage: docker search [OPTIONS] TERM

    Search the Docker Hub for images

      --automated=false    Only show automated builds
      --no-trunc=false     Don't truncate output
      -s, --stars=0        Only displays with at least x stars

See [*Find Public Images on Docker Hub*](
/userguide/dockerrepos/#searching-for-images) for
more details on finding shared images from the command line.

> **Note:**
> Search queries will only return up to 25 results

## start

    Usage: docker start [OPTIONS] CONTAINER [CONTAINER...]

    Start one or more stopped containers

      -a, --attach=false         Attach STDOUT/STDERR and forward signals
      -i, --interactive=false    Attach container's STDIN

## stats

    Usage: docker stats CONTAINER [CONTAINER...]

    Display a live stream of one or more containers' resource usage statistics

      --help=false       Print usage
      --no-stream=false  Disable streaming stats and only pull the first result

Running `docker stats` on multiple containers

    $ docker stats redis1 redis2
    CONTAINER           CPU %               MEM USAGE/LIMIT     MEM %               NET I/O
    redis1              0.07%               796 KB/64 MB        1.21%               788 B/648 B
    redis2              0.07%               2.746 MB/64 MB      4.29%               1.266 KB/648 B


The `docker stats` command will only return a live stream of data for running
containers. Stopped containers will not return any data.

> **Note:**
> If you want more detailed information about a container's resource
> usage, use the API endpoint.

## stop

    Usage: docker stop [OPTIONS] CONTAINER [CONTAINER...]

    Stop a running container by sending SIGTERM and then SIGKILL after a
    grace period

      -t, --time=10      Seconds to wait for stop before killing it

The main process inside the container will receive `SIGTERM`, and after a grace
period, `SIGKILL`.

## tag

    Usage: docker tag [OPTIONS] IMAGE[:TAG] [REGISTRYHOST/][USERNAME/]NAME[:TAG]

    Tag an image into a repository

      -f, --force=false    Force

You can group your images together using names and tags, and then upload them
to [*Share Images via Repositories*](/userguide/dockerrepos/#contributing-to-docker-hub).

## top

    Usage: docker top CONTAINER [ps OPTIONS]

    Display the running processes of a container

## unpause

    Usage: docker unpause CONTAINER [CONTAINER...]

    Unpause all processes within a container

The `docker unpause` command uses the cgroups freezer to un-suspend all
processes in a container.

See the
[cgroups freezer documentation](https://www.kernel.org/doc/Documentation/cgroups/freezer-subsystem.txt)
for further details.

## version

    Usage: docker version

    Show the Docker version information.

Show the Docker version, API version, Git commit, Go version and
OS/architecture of both Docker client and daemon. Example use:

    $ docker version
    Client version: 1.5.0
    Client API version: 1.17
    Go version (client): go1.4.1
    Git commit (client): a8a31ef
    OS/Arch (client): darwin/amd64
    Server version: 1.5.0
    Server API version: 1.17
    Go version (server): go1.4.1
    Git commit (server): a8a31ef
    OS/Arch (server): linux/amd64


## wait

    Usage: docker wait CONTAINER [CONTAINER...]

    Block until a container stops, then print its exit code.

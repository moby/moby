% DOCKER(1) Docker User Manuals
% William Henry
% APRIL 2014
# NAME
docker \- Docker image and container command line interface

# SYNOPSIS
**docker** [OPTIONS] COMMAND [arg...]

# DESCRIPTION
**docker** has two distinct functions. It is used for starting the Docker
daemon and to run the CLI (i.e., to command the daemon to manage images,
containers etc.) So **docker** is both a server, as a daemon, and a client
to the daemon, through the CLI.

To run the Docker daemon you do not specify any of the commands listed below but
must specify the **-d** option.  The other options listed below are for the
daemon only.

The Docker CLI has over 30 commands. The commands are listed below and each has
its own man page which explain usage and arguments.

To see the man page for a command run **man docker <command>**.

# OPTIONS
**-h**, **--help**
  Print usage statement

**--api-cors-header**=""
  Set CORS headers in the remote API. Default is cors disabled. Give urls like "http://foo, http://bar, ...". Give "*" to allow all.

**-b**, **--bridge**=""
  Attach containers to a pre\-existing network bridge; use 'none' to disable container networking

**--bip**=""
  Use the provided CIDR notation address for the dynamically created bridge (docker0); Mutually exclusive of \-b

**-D**, **--debug**=*true*|*false*
  Enable debug mode. Default is false.

**-d**, **--daemon**=*true*|*false*
  Enable daemon mode. Default is false.

**--default-gateway**=""
  IPv4 address of the container default gateway; this address must be part of the bridge subnet (which is defined by \-b or \--bip)

**--default-gateway-v6**=""
  IPv6 address of the container default gateway

**--dns**=""
  Force Docker to use specific DNS servers

**-e**, **--exec-driver**=""
  Force Docker to use specific exec driver. Default is `native`.

**--exec-opt**=[]
  Set exec driver options. See EXEC DRIVER OPTIONS.

**--exec-root**=""
  Path to use as the root of the Docker execdriver. Default is `/var/run/docker`.

**--fixed-cidr**=""
  IPv4 subnet for fixed IPs (e.g., 10.20.0.0/16); this subnet must be nested in the bridge subnet (which is defined by \-b or \-\-bip)

**--fixed-cidr-v6**=""
  IPv6 subnet for global IPv6 addresses (e.g., 2a00:1450::/64)

**-G**, **--group**=""
  Group to assign the unix socket specified by -H when running in daemon mode.
  use '' (the empty string) to disable setting of a group. Default is `docker`.

**-g**, **--graph**=""
  Path to use as the root of the Docker runtime. Default is `/var/lib/docker`.

**-H**, **--host**=[unix:///var/run/docker.sock]: tcp://[host:port] to bind or
unix://[/path/to/socket] to use.
  The socket(s) to bind to in daemon mode specified using one or more
  tcp://host:port, unix:///path/to/socket, fd://* or fd://socketfd.

**--icc**=*true*|*false*
  Allow unrestricted inter\-container and Docker daemon host communication. If disabled, containers can still be linked together using **--link** option (see **docker-run(1)**). Default is true.

**--ip**=""
  Default IP address to use when binding container ports. Default is `0.0.0.0`.

**--ip-forward**=*true*|*false*
  Enables IP forwarding on the Docker host. The default is `true`. This flag interacts with the IP forwarding setting on your host system's kernel. If your system has IP forwarding disabled, this setting enables it. If your system has IP forwarding enabled, setting this flag to `--ip-forward=false` has no effect.

  This setting will also enable IPv6 forwarding if you have both `--ip-forward=true` and `--fixed-cidr-v6` set. Note that this may reject Router Advertisements and interfere with the host's existing IPv6 configuration. For more information, please consult the documentation about "Advanced Networking - IPv6".

**--ip-masq**=*true*|*false*
  Enable IP masquerading for bridge's IP range. Default is true.

**--iptables**=*true*|*false*
  Enable Docker's addition of iptables rules. Default is true.

**--ipv6**=*true*|*false*
  Enable IPv6 support. Default is false. Docker will create an IPv6-enabled bridge with address fe80::1 which will allow you to create IPv6-enabled containers. Use together with `--fixed-cidr-v6` to provide globally routable IPv6 addresses. IPv6 forwarding will be enabled if not used with `--ip-forward=false`. This may collide with your host's current IPv6 settings. For more information please consult the documentation about "Advanced Networking - IPv6".

**-l**, **--log-level**="*debug*|*info*|*warn*|*error*|*fatal*""
  Set the logging level. Default is `info`.

**--label**="[]"
  Set key=value labels to the daemon (displayed in `docker info`)

**--log-driver**="*json-file*|*syslog*|*journald*|*gelf*|*none*"
  Default driver for container logs. Default is `json-file`.
  **Warning**: `docker logs` command works only for `json-file` logging driver.

**--log-opt**=[]
  Logging driver specific options.

**--mtu**=VALUE
  Set the containers network mtu. Default is `0`.

**-p**, **--pidfile**=""
  Path to use for daemon PID file. Default is `/var/run/docker.pid`

**--registry-mirror**=<scheme>://<host>
  Prepend a registry mirror to be used for image pulls. May be specified multiple times.

**-s**, **--storage-driver**=""
  Force the Docker runtime to use a specific storage driver.

**--selinux-enabled**=*true*|*false*
  Enable selinux support. Default is false. SELinux does not presently support the BTRFS storage driver.

**--storage-opt**=[]
  Set storage driver options. See STORAGE DRIVER OPTIONS.

**-tls**=*true*|*false*
  Use TLS; implied by --tlsverify. Default is false.

**-tlsverify**=*true*|*false*
  Use TLS and verify the remote (daemon: verify client, client: verify daemon).
  Default is false.

**--userland-proxy**=*true*|*false*
    Rely on a userland proxy implementation for inter-container and outside-to-container loopback communications. Default is true.

**-v**, **--version**=*true*|*false*
  Print version information and quit. Default is false.

# COMMANDS
**attach**
  Attach to a running container
  See **docker-attach(1)** for full documentation on the **attach** command.

**build**
  Build an image from a Dockerfile
  See **docker-build(1)** for full documentation on the **build** command.

**commit**
  Create a new image from a container's changes
  See **docker-commit(1)** for full documentation on the **commit** command.

**cp**
  Copy files/folders from a container's filesystem to the host
  See **docker-cp(1)** for full documentation on the **cp** command.

**create**
  Create a new container
  See **docker-create(1)** for full documentation on the **create** command.

**diff**
  Inspect changes on a container's filesystem
  See **docker-diff(1)** for full documentation on the **diff** command.

**events**
  Get real time events from the server
  See **docker-events(1)** for full documentation on the **events** command.

**exec**
  Run a command in a running container
  See **docker-exec(1)** for full documentation on the **exec** command.

**export**
  Stream the contents of a container as a tar archive
  See **docker-export(1)** for full documentation on the **export** command.

**history**
  Show the history of an image
  See **docker-history(1)** for full documentation on the **history** command.

**images**
  List images
  See **docker-images(1)** for full documentation on the **images** command.

**import**
  Create a new filesystem image from the contents of a tarball
  See **docker-import(1)** for full documentation on the **import** command.

**info**
  Display system-wide information
  See **docker-info(1)** for full documentation on the **info** command.

**inspect**
  Return low-level information on a container or image
  See **docker-inspect(1)** for full documentation on the **inspect** command.

**kill**
  Kill a running container (which includes the wrapper process and everything
inside it)
  See **docker-kill(1)** for full documentation on the **kill** command.

**load**
  Load an image from a tar archive
  See **docker-load(1)** for full documentation on the **load** command.

**login**
  Register or login to a Docker Registry
  See **docker-login(1)** for full documentation on the **login** command.

**logout**
  Log the user out of a Docker Registry
  See **docker-logout(1)** for full documentation on the **logout** command.

**logs**
  Fetch the logs of a container
  See **docker-logs(1)** for full documentation on the **logs** command.

**pause**
  Pause all processes within a container
  See **docker-pause(1)** for full documentation on the **pause** command.

**port**
  Lookup the public-facing port which is NAT-ed to PRIVATE_PORT
  See **docker-port(1)** for full documentation on the **port** command.

**ps**
  List containers
  See **docker-ps(1)** for full documentation on the **ps** command.

**pull**
  Pull an image or a repository from a Docker Registry
  See **docker-pull(1)** for full documentation on the **pull** command.

**push**
  Push an image or a repository to a Docker Registry
  See **docker-push(1)** for full documentation on the **push** command.

**restart**
  Restart a running container
  See **docker-restart(1)** for full documentation on the **restart** command.

**rm**
  Remove one or more containers
  See **docker-rm(1)** for full documentation on the **rm** command.

**rmi**
  Remove one or more images
  See **docker-rmi(1)** for full documentation on the **rmi** command.

**run**
  Run a command in a new container
  See **docker-run(1)** for full documentation on the **run** command.

**save**
  Save an image to a tar archive
  See **docker-save(1)** for full documentation on the **save** command.

**search**
  Search for an image in the Docker index
  See **docker-search(1)** for full documentation on the **search** command.

**start**
  Start a stopped container
  See **docker-start(1)** for full documentation on the **start** command.

**stats**
  Display a live stream of one or more containers' resource usage statistics
  See **docker-stats(1)** for full documentation on the **stats** command.

**stop**
  Stop a running container
  See **docker-stop(1)** for full documentation on the **stop** command.

**tag**
  Tag an image into a repository
  See **docker-tag(1)** for full documentation on the **tag** command.

**top**
  Lookup the running processes of a container
  See **docker-top(1)** for full documentation on the **top** command.

**unpause**
  Unpause all processes within a container
  See **docker-unpause(1)** for full documentation on the **unpause** command.

**version**
  Show the Docker version information
  See **docker-version(1)** for full documentation on the **version** command.

**wait**
  Block until a container stops, then print its exit code
  See **docker-wait(1)** for full documentation on the **wait** command.

# STORAGE DRIVER OPTIONS

Options to storage backend can be specified with **--storage-opt** flags. The
only backend which currently takes options is *devicemapper*. Therefore use these
flags with **-s=**devicemapper.

Here is the list of *devicemapper* options:

#### dm.basesize
Specifies the size to use when creating the base device, which limits the size
of images and containers. The default value is 10G. Note, thin devices are
inherently "sparse", so a 10G device which is mostly empty doesn't use 10 GB
of space on the pool. However, the filesystem will use more space for the empty
case the larger the device is. **Warning**: This value affects the system-wide
"base" empty filesystem that may already be initialized and inherited by pulled
images.

#### dm.loopdatasize
Specifies the size to use when creating the loopback file for the "data"
device which is used for the thin pool. The default size is 100G. Note that the
file is sparse, so it will not initially take up this much space.

#### dm.loopmetadatasize
Specifies the size to use when creating the loopback file for the "metadadata"
device which is used for the thin pool. The default size is 2G. Note that the
file is sparse, so it will not initially take up this much space.

#### dm.fs
Specifies the filesystem type to use for the base device. The supported
options are "ext4" and "xfs". The default is "ext4"

#### dm.mkfsarg
Specifies extra mkfs arguments to be used when creating the base device.

#### dm.mountopt
Specifies extra mount options used when mounting the thin devices.

#### dm.datadev
Specifies a custom blockdevice to use for data for the thin pool.

If using a block device for device mapper storage, ideally both datadev and
metadatadev should be specified to completely avoid using the loopback device.

#### dm.metadatadev
Specifies a custom blockdevice to use for metadata for the thin pool.

For best performance the metadata should be on a different spindle than the
data, or even better on an SSD.

If setting up a new metadata pool it is required to be valid. This can be
achieved by zeroing the first 4k to indicate empty metadata, like this:

    dd if=/dev/zero of=/dev/metadata_dev bs=4096 count=1

#### dm.blocksize
Specifies a custom blocksize to use for the thin pool. The default blocksize
is 64K.

#### dm.blkdiscard
Enables or disables the use of blkdiscard when removing devicemapper devices.
This is enabled by default (only) if using loopback devices and is required to
resparsify the loopback file on image/container removal.

Disabling this on loopback can lead to *much* faster container removal times,
but will prevent the space used in `/var/lib/docker` directory from being returned to
the system for other use when containers are removed.

# EXAMPLES
Launching docker daemon with *devicemapper* backend with particular block devices
for data and metadata:

    docker -d -s=devicemapper \
      --storage-opt dm.datadev=/dev/vdb \
      --storage-opt dm.metadatadev=/dev/vdc \
      --storage-opt dm.basesize=20G

# EXEC DRIVER OPTIONS

Use the **--exec-opt** flags to specify options to the exec-driver. The only
driver that accepts this flag is the *native* (libcontainer) driver. As a
result, you must also specify **-s=**native for this option to have effect. The 
following is the only *native* option:

#### native.cgroupdriver
Specifies the management of the container's `cgroups`. You can specify 
`cgroupfs` or `systemd`. If you specify `systemd` and it is not available, the 
system uses `cgroupfs`.

#### Client
For specific client examples please see the man page for the specific Docker
command. For example:

    man docker-run

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com) based on docker.com source material and internal work.

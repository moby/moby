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
**-D**=*true*|*false*
   Enable debug mode. Default is false.

**-H**, **--host**=[unix:///var/run/docker.sock]: tcp://[host:port] to bind or
unix://[/path/to/socket] to use.
   The socket(s) to bind to in daemon mode specified using one or more
   tcp://host:port, unix:///path/to/socket, fd://* or fd://socketfd.

**--api-enable-cors**=*true*|*false*
  Enable CORS headers in the remote API. Default is false.

**-b**=""
  Attach containers to a pre\-existing network bridge; use 'none' to disable container networking

**--bip**=""
  Use the provided CIDR notation address for the dynamically created bridge (docker0); Mutually exclusive of \-b

**-d**=*true*|*false*
  Enable daemon mode. Default is false.

**--dns**=""
  Force Docker to use specific DNS servers

**-g**=""
  Path to use as the root of the Docker runtime. Default is `/var/lib/docker`.


**--fixed-cidr**=""
  IPv4 subnet for fixed IPs (ex: 10.20.0.0/16); this subnet must be nested in the bridge subnet (which is defined by \-b or \-\-bip)

**--icc**=*true*|*false*
  Allow unrestricted inter\-container and Docker daemon host communication. If disabled, containers can still be linked together using **--link** option (see **docker-run(1)**). Default is true.

**--ip**=""
  Default IP address to use when binding container ports. Default is `0.0.0.0`.

**--ip-masq**=*true*|*false*
  Enable IP masquerading for bridge's IP range. Default is true.

**--iptables**=*true*|*false*
  Disable Docker's addition of iptables rules. Default is true.

**-l**, **--log-level**="*debug*|*info*|*error*|*fatal*""
  Set the logging level. Default is `info`.

**--label**="[]"
  Set key=value labels to the daemon (displayed in `docker info`)

**--mtu**=VALUE
  Set the containers network mtu. Default is `1500`.

**-p**=""
  Path to use for daemon PID file. Default is `/var/run/docker.pid`

**--registry-mirror**=<scheme>://<host>
  Prepend a registry mirror to be used for image pulls. May be specified multiple times.

**-s**=""
  Force the Docker runtime to use a specific storage driver.

**--storage-opt**=[]
  Set storage driver options. See STORAGE DRIVER OPTIONS.

**-v**=*true*|*false*
  Print version information and quit. Default is false.

**--selinux-enabled**=*true*|*false*
  Enable selinux support. Default is false. SELinux does not presently support the BTRFS storage driver.

# COMMANDS
**docker-attach(1)**
  Attach to a running container

**docker-build(1)**
  Build an image from a Dockerfile

**docker-commit(1)**
  Create a new image from a container's changes

**docker-cp(1)**
  Copy files/folders from a container's filesystem to the host at path

**docker-create(1)**
  Create a new container

**docker-diff(1)**
  Inspect changes on a container's filesystem

**docker-events(1)**
  Get real time events from the server

**docker-exec(1)**
  Run a command in a running container

**docker-export(1)**
  Stream the contents of a container as a tar archive

**docker-history(1)**
  Show the history of an image

**docker-images(1)**
  List images

**docker-import(1)**
  Create a new filesystem image from the contents of a tarball

**docker-info(1)**
  Display system-wide information

**docker-inspect(1)**
  Return low-level information on a container

**docker-kill(1)**
  Kill a running container (which includes the wrapper process and everything
inside it)

**docker-load(1)**
  Load an image from a tar archive

**docker-login(1)**
  Register or Login to a Docker registry server

**docker-logout(1)**
  Log the user out of a Docker registry server

**docker-logs(1)**
  Fetch the logs of a container

**docker-pause(1)**
  Pause all processes within a container

**docker-port(1)**
  Lookup the public-facing port which is NAT-ed to PRIVATE_PORT

**docker-ps(1)**
  List containers

**docker-pull(1)**
  Pull an image or a repository from a Docker registry server

**docker-push(1)**
  Push an image or a repository to a Docker registry server

**docker-restart(1)**
  Restart a running container

**docker-rm(1)**
  Remove one or more containers

**docker-rmi(1)**
  Remove one or more images

**docker-run(1)**
  Run a command in a new container

**docker-save(1)**
  Save an image to a tar archive

**docker-search(1)**
  Search for an image in the Docker index

**docker-start(1)**
  Start a stopped container

**docker-stop(1)**
  Stop a running container

**docker-tag(1)**
  Tag an image into a repository

**docker-top(1)**
  Lookup the running processes of a container

**docker-unpause(1)**
  Unpause all processes within a container

**docker-version(1)**
  Show the Docker version information

**docker-wait(1)**
  Block until a container stops, then print its exit code

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
res-parsify the loopback file on image/container removal.

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

#### Client
For specific client examples please see the man page for the specific Docker
command. For example:

    man docker run

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com) based on docker.com source material and internal work.

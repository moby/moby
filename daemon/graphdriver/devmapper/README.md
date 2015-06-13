## devicemapper - a storage backend based on Device Mapper

### Theory of operation

The device mapper graphdriver uses the device mapper thin provisioning
module (dm-thinp) to implement CoW snapshots. For each devicemapper
graph location (typically `/var/lib/docker/devicemapper`, $graph below)
a thin pool is created based on two block devices, one for data and
one for metadata.  By default these block devices are created
automatically by using loopback mounts of automatically created sparse
files.

The default loopback files used are `$graph/devicemapper/data` and
`$graph/devicemapper/metadata`. Additional metadata required to map
from docker entities to the corresponding devicemapper volumes is
stored in the `$graph/devicemapper/json` file (encoded as Json).

In order to support multiple devicemapper graphs on a system, the thin
pool will be named something like: `docker-0:33-19478248-pool`, where
the `0:33` part is the minor/major device nr and `19478248` is the
inode number of the $graph directory.

On the thin pool, docker automatically creates a base thin device,
called something like `docker-0:33-19478248-base` of a fixed
size. This is automatically formatted with an empty filesystem on
creation. This device is the base of all docker images and
containers. All base images are snapshots of this device and those
images are then in turn used as snapshots for other images and
eventually containers.

### Information on `docker info`

As of docker-1.4.1, `docker info` when using the `devicemapper` storage driver
will display something like:

	$ sudo docker info
	[...]
	Storage Driver: devicemapper
	 Pool Name: docker-253:1-17538953-pool
	 Pool Blocksize: 65.54 kB
	 Data file: /dev/loop4
	 Metadata file: /dev/loop4
	 Data Space Used: 2.536 GB
	 Data Space Total: 107.4 GB
	 Data Space Available: 104.8 GB
	 Metadata Space Used: 7.93 MB
	 Metadata Space Total: 2.147 GB
	 Metadata Space Available: 2.14 GB
	 Udev Sync Supported: true
	 Data loop file: /home/docker/devicemapper/devicemapper/data
	 Metadata loop file: /home/docker/devicemapper/devicemapper/metadata
	 Library Version: 1.02.82-git (2013-10-04)
	[...]

#### status items

Each item in the indented section under `Storage Driver: devicemapper` are
status information about the driver.
 *  `Pool Name` name of the devicemapper pool for this driver.
 *  `Pool Blocksize` tells the blocksize the thin pool was initialized with. This only changes on creation.
 *  `Data file` blockdevice file used for the devicemapper data
 *  `Metadata file` blockdevice file used for the devicemapper metadata
 *  `Data Space Used` tells how much of `Data file` is currently used
 *  `Data Space Total` tells max size the `Data file`
 *  `Data Space Available` tells how much free space there is in the `Data file`. If you are using a loop device this will report the actual space available to the loop device on the underlying filesystem.
 *  `Metadata Space Used` tells how much of `Metadata file` is currently used
 *  `Metadata Space Total` tells max size the `Metadata file`
 *  `Metadata Space Available` tells how much free space there is in the `Metadata file`. If you are using a loop device this will report the actual space available to the loop device on the underlying filesystem.
 *  `Udev Sync Supported` tells whether devicemapper is able to sync with Udev. Should be `true`.
 *  `Data loop file` file attached to `Data file`, if loopback device is used
 *  `Metadata loop file` file attached to `Metadata file`, if loopback device is used
 *  `Library Version` from the libdevmapper used

### options

The devicemapper backend supports some options that you can specify
when starting the docker daemon using the `--storage-opt` flags.
This uses the `dm` prefix and would be used something like `docker -d --storage-opt dm.foo=bar`.

Here is the list of supported options:

 *  `dm.basesize`

    Specifies the size to use when creating the base device, which
    limits the size of images and containers. The default value is
    10G. Note, thin devices are inherently "sparse", so a 10G device
    which is mostly empty doesn't use 10 GB of space on the
    pool. However, the filesystem will use more space for the empty
    case the larger the device is. **Warning**: This value affects the
    system-wide "base" empty filesystem that may already be
    initialized and inherited by pulled images.  Typically, a change
    to this value will require additional steps to take effect: 1)
    stop `docker -d`, 2) `rm -rf /var/lib/docker`, 3) start `docker -d`.

    Example use:

    ``docker -d --storage-opt dm.basesize=20G``

 *  `dm.loopdatasize`

    Specifies the size to use when creating the loopback file for the
    "data" device which is used for the thin pool. The default size is
    100G. Note that the file is sparse, so it will not initially take
    up this much space.

    Example use:

    ``docker -d --storage-opt dm.loopdatasize=200G``

 *  `dm.loopmetadatasize`

    Specifies the size to use when creating the loopback file for the
    "metadadata" device which is used for the thin pool. The default size is
    2G. Note that the file is sparse, so it will not initially take
    up this much space.

    Example use:

    ``docker -d --storage-opt dm.loopmetadatasize=4G``

 *  `dm.fs`

    Specifies the filesystem type to use for the base device. The supported
    options are "ext4" and "xfs". The default is "ext4"

    Example use:

    ``docker -d --storage-opt dm.fs=xfs``

 *  `dm.mkfsarg`

    Specifies extra mkfs arguments to be used when creating the base device.

    Example use:

    ``docker -d --storage-opt "dm.mkfsarg=-O ^has_journal"``

 *  `dm.mountopt`

    Specifies extra mount options used when mounting the thin devices.

    Example use:

    ``docker -d --storage-opt dm.mountopt=nodiscard``

 *  `dm.thinpooldev`

    Specifies a custom blockdevice to use for the thin pool.

    If using a block device for device mapper storage, ideally lvm2
    would be used to create/manage the thin-pool volume that is then
    handed to docker to exclusively create/manage the thin and thin
    snapshot volumes needed for its containers.  Managing the thin-pool
    outside of docker makes for the most feature-rich method of having
    docker utilize device mapper thin provisioning as the backing
    storage for docker's containers.  lvm2-based thin-pool management
    feature highlights include: automatic or interactive thin-pool
    resize support, dynamically change thin-pool features, automatic
    thinp metadata checking when lvm2 activates the thin-pool, etc.

    Example use:

    ``docker -d --storage-opt dm.thinpooldev=/dev/mapper/thin-pool``

 *  `dm.datadev`

    Specifies a custom blockdevice to use for data for the thin pool.

    If using a block device for device mapper storage, ideally both
    datadev and metadatadev should be specified to completely avoid
    using the loopback device.

    Example use:

    ``docker -d --storage-opt dm.datadev=/dev/sdb1 --storage-opt dm.metadatadev=/dev/sdc1``

 *  `dm.metadatadev`

    Specifies a custom blockdevice to use for metadata for the thin
    pool.

    For best performance the metadata should be on a different spindle
    than the data, or even better on an SSD.

    If setting up a new metadata pool it is required to be valid. This
    can be achieved by zeroing the first 4k to indicate empty
    metadata, like this:

    ``dd if=/dev/zero of=$metadata_dev bs=4096 count=1``

    Example use:

    ``docker -d --storage-opt dm.datadev=/dev/sdb1 --storage-opt dm.metadatadev=/dev/sdc1``

 *  `dm.blocksize`

    Specifies a custom blocksize to use for the thin pool.  The default
    blocksize is 64K.

    Example use:

    ``docker -d --storage-opt dm.blocksize=512K``

 *  `dm.blkdiscard`

    Enables or disables the use of blkdiscard when removing
    devicemapper devices. This is enabled by default (only) if using
    loopback devices and is required to resparsify the loopback file
    on image/container removal.

    Disabling this on loopback can lead to *much* faster container
    removal times, but will make the space used in /var/lib/docker
    directory not be returned to the system for other use when
    containers are removed.

    Example use:

    ``docker -d --storage-opt dm.blkdiscard=false``

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

    > **Note**: The ideal is to pursue a `docker` daemon and environment that
    > does support synchronizing with `udev`. For further discussion on this
    > topic, see [docker#4036](https://github.com/docker/docker/issues/4036).
    > Otherwise, set this flag for migrating existing Docker daemons to a
    > daemon with a supported environment.

 *  `dm.use_deferred_removal`

    Enables use of deferred device removal if libdm and kernel driver
    support the mechanism.

    Deferred device removal means that if device is busy when devices is
    being removed/deactivated, then a deferred removal is scheduled on
    device. And devices automatically goes away when last user of device
    exits.

    For example, when container exits, its associated thin device is
    removed. If that devices has leaked into some other mount namespace
    can can't be removed now, container exit will still be successful
    and this option will just schedule device for deferred removal and
    will not wait in a loop trying to remove a busy device.

    Example use:

    ``docker -d --storage-opt dm.use_deferred_removal=true``


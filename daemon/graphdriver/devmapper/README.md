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

    ``dd if=/dev/zero of=$metadata_dev bs=4096 count=1```

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
    loopback devices and is required to res-parsify the loopback file
    on image/container removal.

    Disabling this on loopback can lead to *much* faster container
    removal times, but will make the space used in /var/lib/docker
    directory not be returned to the system for other use when
    containers are removed.

    Example use:

    ``docker -d --storage-opt dm.blkdiscard=false``

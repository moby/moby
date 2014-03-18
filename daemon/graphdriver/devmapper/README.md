## devicemapper - a storage backend based on Device Mapper

### Theory of operation

The device mapper graphdriver uses the device mapper thin provisioning
module (dm-thinp) to implement CoW snapshots. For each devicemapper
graph locaion (typically `/var/lib/docker/devicemapper`, $graph below)
a thin pool is created based on two block devices, one for data and
one for metadata.  By default these block devices are created
automatically by using loopback mounts of automatically creates sparse
files.

The default loopback files used are `$graph/devicemapper/data` and
`$graph/devicemapper/metadata`. Additional metadata required to map
from docker entities to the corresponding devicemapper volumes is
stored in the `$graph/devicemapper/json` file (encoded as Json).

In order to support multiple devicemapper graphs on a system the thin
pool will be named something like: `docker-0:33-19478248-pool`, where
the `0:30` part is the minor/major device nr and `19478248` is the
inode number of the $graph directory.

On the thin pool docker automatically creates a base thin device,
called something like `docker-0:33-19478248-base` of a fixed
size. This is automatically formated on creation and contains just an
empty filesystem. This device is the base of all docker images and
containers. All base images are snapshots of this device and those
images are then in turn used as snapshots for other images and
eventually containers.

### options

The devicemapper backend supports some options that you can specify
when starting the docker daemon using the --storage-opt flags.
This uses the `dm` prefix and would be used somthing like `docker -d --storage-opt dm.foo=bar`.

Here is the list of supported options:

 *  `dm.basesize`

    Specifies the size to use when creating the base device, which
    limits the size of images and containers. The default value is
    10G. Note, thin devices are inherently "sparse", so a 10G device
    which is mostly empty doesn't use 10 GB of space on the
    pool. However, the filesystem will use more space for the empty
    case the larger the device is.

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

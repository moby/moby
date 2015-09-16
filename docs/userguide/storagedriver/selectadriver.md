<!--[metadata]>
+++
title = "Select a storage driver"
description = "Learn how select the proper storage driver for your container."
keywords = ["container, storage, driver, AUFS, btfs, devicemapper,zvfs"]
[menu.main]
parent = "mn_storage_docker"
weight = -1
+++
<![end-metadata]-->

# Select a storage driver

This page describes Docker's storage driver feature. It lists the storage
driver's that Docker supports and the basic commands associated with managing them. Finally, this page provides guidance on choosing a storage driver.

The material on this page is intended for readers who already have an [understanding of the storage driver technology](imagesandcontainers.md).

## A pluggable storage driver architecture

The Docker has a pluggable storage driver architecture.  This gives you the flexibility to "plug in" the storage driver is best for your environment and use-case. Each Docker storage driver is based on a Linux filesystem or volume manager. Further, each storage driver is free to implement the management of image layers and the container layer in it's own unique way. This means some storage drivers perform better than others in different circumstances.

Once you decide which driver is best, you set this driver on the Docker daemon at start time. As a result, the Docker daemon can only run one storage driver, and all containers created by that daemon instance use the same storage driver. The table below shows the supported storage driver technologies and the driver names:

|Technology    |Storage driver name  |
|--------------|---------------------|
|OverlayFS     |`overlay`            |
|AUFS          |`aufs`               |
|BTRFS         |`btrfs`              |
|Device Maper  |`devicemapper`       |
|VFS*          |`vfs`                |
|ZFS           |`zfs`                |

To find out which storage driver is set on the daemon , you use the `docker info` command:

    $ docker info
    Containers: 0
    Images: 0
    Storage Driver: overlay
     Backing Filesystem: extfs
    Execution Driver: native-0.2
    Logging Driver: json-file
    Kernel Version: 3.19.0-15-generic
    Operating System: Ubuntu 15.04
    ... output truncated ...

The `info` subcommand reveals that the Docker daemon is using the `overlay` storage driver with a `Backing Filesystem` value of `extfs`. The `extfs` value means that the `overlay` storage driver is operating on top of an existing (ext) filesystem. The backing filesystem refers to the filesystem that was used to create the Docker host's local storage area under `/var/lib/docker`.

Which storage driver you use, in part, depends on the backing filesystem you plan to use for your Docker host's local storage area. Some storage drivers can operate on top of different backing filesystems. However, other storage drivers require the backing filesystem to be the same as the storage driver. For example, the `btrfs` storage driver on a `btrfs` backing filesystem. The following table lists each storage driver and whether it must match the host's backing file system:

    |Storage driver |Must match backing filesystem |
    |---------------|------------------------------|
    |overlay        |No                            |
    |aufs           |No                            |
    |btrfs          |Yes                           |
    |devicemapper   |No                            |
    |vfs*           |No                            |
    |zfs            |Yes                           |


You pass the `--storage-driver=<name>` option to the `docker daemon` command line or by setting the option on the `DOCKER_OPTS` line in `/etc/defaults/docker` file.

The following command shows how to start the Docker daemon with the `devicemapper` storage driver using the `docker daemon` command:

    $ docker daemon --storage-driver=devicemapper &

    $ docker info
    Containers: 0
    Images: 0
    Storage Driver: devicemapper
     Pool Name: docker-252:0-147544-pool
     Pool Blocksize: 65.54 kB
     Backing Filesystem: extfs
     Data file: /dev/loop0
     Metadata file: /dev/loop1
     Data Space Used: 1.821 GB
     Data Space Total: 107.4 GB
     Data Space Available: 3.174 GB
     Metadata Space Used: 1.479 MB
     Metadata Space Total: 2.147 GB
     Metadata Space Available: 2.146 GB
     Udev Sync Supported: true
     Deferred Removal Enabled: false
     Data loop file: /var/lib/docker/devicemapper/devicemapper/data
     Metadata loop file: /var/lib/docker/devicemapper/devicemapper/metadata
     Library Version: 1.02.90 (2014-09-01)
    Execution Driver: native-0.2
    Logging Driver: json-file
    Kernel Version: 3.19.0-15-generic
    Operating System: Ubuntu 15.04
    <output truncated>

Your choice of storage driver can affect the performance of your containerized applications. So it's important to understand the different storage driver options available and select the right one for your application. Later, in this page you'll find some advice for choosing an appropriate driver.

## Shared storage systems and the storage driver

Many enterprises consume storage from shared storage systems such as SAN and NAS arrays. These often provide increased performance and availability, as well as advanced features such as thin provisioning, deduplication and compression.

The Docker storage driver and data volumes can both operate on top of storage provided by shared storage systems. This allows Docker to leverage the increased performance and availability these systems provide. However, Docker does not integrate with these underlying systems.

Remember that each Docker storage driver is based on a Linux filesystem or volume manager. Be sure to follow existing best practices for operating your storage driver (filesystem or volume manager) on top of your shared storage system. For example, if using the ZFS storage driver on top of *XYZ* shared storage system, be sure to follow best practices for operating ZFS filesystems on top of XYZ shared storage system.

## Which storage driver should you choose?

As you might expect, the answer to this question is "it depends". While there are some clear cases where one particular storage driver outperforms other for certain workloads, you should factor all of the following into your decision:

Choose a storage driver that you and your team/organization are comfortable with.  Consider how much experience you have with a particular storage driver. There is no substitute for experience and it is rarely a good idea to try something brand new in production. That's what labs and laptops are for!

If your Docker infrastructure is under support contracts, choose an option that will get you good support. You probably don't want to go with a solution that your support partners have little or no experience with.

Whichever driver you choose, make sure it has strong community support and momentum. This is important because storage driver development in the Docker project relies on the community as much as the Docker staff to thrive.


## Related information

* [Understand images, containers, and storage drivers](imagesandcontainers.md)
* [AUFS storage driver in practice](aufs-driver.md)
* [BTRFS storage driver in practice](btrfs-driver.md)
* [Device Mapper storage driver in practice](device-mapper-driver.md)

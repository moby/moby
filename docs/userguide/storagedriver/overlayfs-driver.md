<!--[metadata]>
+++
title = "OverlayFS storage in practice"
description = "Learn how to optimize your use of OverlayFS driver."
keywords = ["container, storage, driver, OverlayFS "]
[menu.main]
parent = "engine_driver"
+++
<![end-metadata]-->

# Docker and OverlayFS in practice

OverlayFS is a modern *union filesystem* that is similar to AUFS. In comparison
 to AUFS, OverlayFS:

* has a simpler design
* has been in the mainline Linux kernel since version 3.18
* is potentially faster

As a result, OverlayFS is rapidly gaining popularity in the Docker community 
and is seen by many as a natural successor to AUFS. As promising as OverlayFS 
is, it is still relatively young. Therefore caution should be taken before 
using it in production Docker environments.

Docker's `overlay` storage driver leverages several OverlayFS features to build
 and manage the on-disk structures of images and containers.

Since version 1.12, Docker also provides `overlay2` storage driver which is much
more efficient than `overlay` in terms of inode utilization. The `overlay2`
driver is only compatible with Linux kernel 4.0 and later.

For comparison between `overlay` vs `overlay2`, please also refer to [Select a
storage driver](selectadriver.md#overlay-vs-overlay2).

>**Note**: Since it was merged into the mainline kernel, the OverlayFS *kernel 
>module* was renamed from "overlayfs" to "overlay". As a result you may see the
> two terms used interchangeably in some documentation. However, this document 
> uses  "OverlayFS" to refer to the overall filesystem, and `overlay`/`overlay2`
> to refer to Docker's storage-drivers.

## Image layering and sharing with OverlayFS (`overlay`)

OverlayFS takes two directories on a single Linux host, layers one on top of 
the other, and provides a single unified view. These directories are often 
referred to as *layers* and the technology used to layer them is known as a 
*union mount*. The OverlayFS terminology is "lowerdir" for the bottom layer and
 "upperdir" for the top layer. The unified view is exposed through its own 
directory called "merged".

The diagram below shows how a Docker image and a Docker container are layered. 
The image layer is the "lowerdir" and the container layer is the "upperdir". 
The unified view is exposed through a directory called "merged" which is 
effectively the containers mount point. The diagram shows how Docker constructs
 map to OverlayFS constructs.

![](images/overlay_constructs.jpg)

Notice how the image layer and container layer can contain the same files. When
 this happens, the files in the container layer ("upperdir") are dominant and 
obscure the existence of the same files in the image layer ("lowerdir"). The 
container mount ("merged") presents the unified view.

The `overlay` driver only works with two layers. This means that multi-layered
images cannot be implemented as multiple OverlayFS layers. Instead, each image
layer is implemented as its own directory under `/var/lib/docker/overlay`.  Hard
links are then used as a space-efficient way to reference data shared with lower
layers. As of Docker 1.10, image layer IDs no longer correspond to directory
names in `/var/lib/docker/`

To create a container, the `overlay` driver combines the directory representing
 the image's top layer plus a new directory for the container. The image's top 
layer is the "lowerdir" in the overlay and read-only. The new directory for the
 container is the "upperdir" and is writable.

### Example: Image and container on-disk constructs (`overlay`)

The following `docker pull` command shows a Docker host with downloading a 
Docker image comprising five layers.

    $ sudo docker pull ubuntu

    Using default tag: latest
    latest: Pulling from library/ubuntu

    5ba4f30e5bea: Pull complete
    9d7d19c9dc56: Pull complete
    ac6ad7efd0f9: Pull complete
    e7491a747824: Pull complete
    a3ed95caeb02: Pull complete
    Digest: sha256:46fb5d001b88ad904c5c732b086b596b92cfb4a4840a3abd0e35dbb6870585e4
    Status: Downloaded newer image for ubuntu:latest

Each image layer has its own directory under `/var/lib/docker/overlay/`. This 
is where the contents of each image layer are stored. 

The output of the command below shows the five directories that store the 
contents of each image layer just pulled. However, as can be seen, the image 
layer IDs do not match the directory names in `/var/lib/docker/overlay`. This 
is normal behavior in Docker 1.10 and later.

    $ ls -l /var/lib/docker/overlay/

    total 20
    drwx------ 3 root root 4096 Jun 20 16:11 38f3ed2eac129654acef11c32670b534670c3a06e483fce313d72e3e0a15baa8
    drwx------ 3 root root 4096 Jun 20 16:11 55f1e14c361b90570df46371b20ce6d480c434981cbda5fd68c6ff61aa0a5358
    drwx------ 3 root root 4096 Jun 20 16:11 824c8a961a4f5e8fe4f4243dab57c5be798e7fd195f6d88ab06aea92ba931654
    drwx------ 3 root root 4096 Jun 20 16:11 ad0fe55125ebf599da124da175174a4b8c1878afe6907bf7c78570341f308461
    drwx------ 3 root root 4096 Jun 20 16:11 edab9b5e5bf73f2997524eebeac1de4cf9c8b904fa8ad3ec43b3504196aa3801

The image layer directories contain the files unique to that layer as well as 
hard links to the data that is shared with lower layers. This allows for 
efficient use of disk space.

    $ ls -i /var/lib/docker/overlay/38f3ed2eac129654acef11c32670b534670c3a06e483fce313d72e3e0a15baa8/root/bin/ls

    19793696 /var/lib/docker/overlay/38f3ed2eac129654acef11c32670b534670c3a06e483fce313d72e3e0a15baa8/root/bin/ls

    $ ls -i /var/lib/docker/overlay/55f1e14c361b90570df46371b20ce6d480c434981cbda5fd68c6ff61aa0a5358/root/bin/ls

    19793696 /var/lib/docker/overlay/55f1e14c361b90570df46371b20ce6d480c434981cbda5fd68c6ff61aa0a5358/root/bin/ls

Containers also exist on-disk in the Docker host's filesystem under 
`/var/lib/docker/overlay/`. If you inspect the directory relating to a running 
container using the `ls -l` command, you find the following file and 
directories.

    $ ls -l /var/lib/docker/overlay/<directory-of-running-container>

    total 16
    -rw-r--r-- 1 root root   64 Jun 20 16:39 lower-id
    drwxr-xr-x 1 root root 4096 Jun 20 16:39 merged
    drwxr-xr-x 4 root root 4096 Jun 20 16:39 upper
    drwx------ 3 root root 4096 Jun 20 16:39 work

These four filesystem objects are all artifacts of OverlayFS. The "lower-id" 
file contains the ID of the top layer of the image the container is based on. 
This is used by OverlayFS as the "lowerdir".

    $ cat /var/lib/docker/overlay/ec444863a55a9f1ca2df72223d459c5d940a721b2288ff86a3f27be28b53be6c/lower-id

    55f1e14c361b90570df46371b20ce6d480c434981cbda5fd68c6ff61aa0a5358

The "upper" directory is the containers read-write layer. Any changes made to 
the container are written to this directory.

The "merged" directory is effectively the containers mount point. This is where
 the unified view of the image ("lowerdir") and container ("upperdir") is 
exposed. Any changes written to the container are immediately reflected in this
 directory.

The "work" directory is required for OverlayFS to function. It is used for 
things such as *copy_up* operations.

You can verify all of these constructs from the output of the `mount` command. 
(Ellipses and line breaks are used in the output below to enhance readability.)

    $ mount | grep overlay

    overlay on /var/lib/docker/overlay/ec444863a55a.../merged
    type overlay (rw,relatime,lowerdir=/var/lib/docker/overlay/55f1e14c361b.../root,
    upperdir=/var/lib/docker/overlay/ec444863a55a.../upper,
    workdir=/var/lib/docker/overlay/ec444863a55a.../work)

The output reflects that the overlay is mounted as read-write ("rw").


## Image layering and sharing with OverlayFS (`overlay2`)

While the `overlay` driver only works with a single lower OverlayFS layer and
hence requires hard links for implementation of multi-layered images, the
`overlay2` driver natively supports multiple lower OverlayFS layers (up to 128).

Hence the `overlay2` driver offers better performance for layer-related docker commands (e.g. `docker build` and `docker commit`), and consumes fewer inodes than the `overlay` driver.

### Example: Image and container on-disk constructs (`overlay2`)

After downloading a five-layer image using `docker pull ubuntu`, you can see
six directories under `/var/lib/docker/overlay2`.

    $ ls -l /var/lib/docker/overlay2

    total 24
    drwx------ 5 root root 4096 Jun 20 07:36 223c2864175491657d238e2664251df13b63adb8d050924fd1bfcdb278b866f7
    drwx------ 3 root root 4096 Jun 20 07:36 3a36935c9df35472229c57f4a27105a136f5e4dbef0f87905b2e506e494e348b
    drwx------ 5 root root 4096 Jun 20 07:36 4e9fa83caff3e8f4cc83693fa407a4a9fac9573deaf481506c102d484dd1e6a1
    drwx------ 5 root root 4096 Jun 20 07:36 e8876a226237217ec61c4baf238a32992291d059fdac95ed6303bdff3f59cff5
    drwx------ 5 root root 4096 Jun 20 07:36 eca1e4e1694283e001f200a667bb3cb40853cf2d1b12c29feda7422fed78afed
    drwx------ 2 root root 4096 Jun 20 07:36 l

The "l" directory contains shortened layer identifiers as symbolic links.  These
shortened identifiers are used for avoid hitting the page size limitation on
mount arguments.

    $ ls -l /var/lib/docker/overlay2/l

    total 20
    lrwxrwxrwx 1 root root 72 Jun 20 07:36 6Y5IM2XC7TSNIJZZFLJCS6I4I4 -> ../3a36935c9df35472229c57f4a27105a136f5e4dbef0f87905b2e506e494e348b/diff
    lrwxrwxrwx 1 root root 72 Jun 20 07:36 B3WWEFKBG3PLLV737KZFIASSW7 -> ../4e9fa83caff3e8f4cc83693fa407a4a9fac9573deaf481506c102d484dd1e6a1/diff
    lrwxrwxrwx 1 root root 72 Jun 20 07:36 JEYMODZYFCZFYSDABYXD5MF6YO -> ../eca1e4e1694283e001f200a667bb3cb40853cf2d1b12c29feda7422fed78afed/diff
    lrwxrwxrwx 1 root root 72 Jun 20 07:36 NFYKDW6APBCCUCTOUSYDH4DXAT -> ../223c2864175491657d238e2664251df13b63adb8d050924fd1bfcdb278b866f7/diff
    lrwxrwxrwx 1 root root 72 Jun 20 07:36 UL2MW33MSE3Q5VYIKBRN4ZAGQP -> ../e8876a226237217ec61c4baf238a32992291d059fdac95ed6303bdff3f59cff5/diff

The lowerest layer contains the "link" file which contains the name of the shortened
identifier, and the "diff" directory which contains the contents.

    $ ls /var/lib/docker/overlay2/3a36935c9df35472229c57f4a27105a136f5e4dbef0f87905b2e506e494e348b/

    diff  link

    $ cat /var/lib/docker/overlay2/3a36935c9df35472229c57f4a27105a136f5e4dbef0f87905b2e506e494e348b/link

    6Y5IM2XC7TSNIJZZFLJCS6I4I4

    $ ls  /var/lib/docker/overlay2/3a36935c9df35472229c57f4a27105a136f5e4dbef0f87905b2e506e494e348b/diff

    bin  boot  dev  etc  home  lib  lib64  media  mnt  opt  proc  root  run  sbin  srv  sys  tmp  usr  var

The second layer contains the "lower" file for denoting the layer composition,
and the "diff" directory for the layer contents.  It also contains the "merged" and
the "work" directories.

    $ ls /var/lib/docker/overlay2/223c2864175491657d238e2664251df13b63adb8d050924fd1bfcdb278b866f7

    diff  link  lower  merged  work

    $ cat /var/lib/docker/overlay2/223c2864175491657d238e2664251df13b63adb8d050924fd1bfcdb278b866f7/lower

    l/6Y5IM2XC7TSNIJZZFLJCS6I4I4

    $ ls /var/lib/docker/overlay2/223c2864175491657d238e2664251df13b63adb8d050924fd1bfcdb278b866f7/diff/

    etc  sbin  usr  var

A directory for running container have similar files and directories as well.
Note that the lower list is separated by ':', and ordered from highest layer to lower.

    $ ls -l /var/lib/docker/overlay/<directory-of-running-container>

    $ cat /var/lib/docker/overlay/<directory-of-running-container>/lower

    l/DJA75GUWHWG7EWICFYX54FIOVT:l/B3WWEFKBG3PLLV737KZFIASSW7:l/JEYMODZYFCZFYSDABYXD5MF6YO:l/UL2MW33MSE3Q5VYIKBRN4ZAGQP:l/NFYKDW6APBCCUCTOUSYDH4DXAT:l/6Y5IM2XC7TSNIJZZFLJCS6I4I4

The result of `mount` is as follows:

    $ mount | grep overlay

    overlay on /var/lib/docker/overlay2/9186877cdf386d0a3b016149cf30c208f326dca307529e646afce5b3f83f5304/merged
    type overlay (rw,relatime,
    lowerdir=l/DJA75GUWHWG7EWICFYX54FIOVT:l/B3WWEFKBG3PLLV737KZFIASSW7:l/JEYMODZYFCZFYSDABYXD5MF6YO:l/UL2MW33MSE3Q5VYIKBRN4ZAGQP:l/NFYKDW6APBCCUCTOUSYDH4DXAT:l/6Y5IM2XC7TSNIJZZFLJCS6I4I4,
    upperdir=9186877cdf386d0a3b016149cf30c208f326dca307529e646afce5b3f83f5304/diff,
    workdir=9186877cdf386d0a3b016149cf30c208f326dca307529e646afce5b3f83f5304/work)

## Container reads and writes with overlay

Consider three scenarios where a container opens a file for read access with 
overlay.

- **The file does not exist in the container layer**. If a container opens a 
file for read access and the file does not already exist in the container 
("upperdir") it is read from the image ("lowerdir"). This should incur very 
little performance overhead.

- **The file only exists in the container layer**. If a container opens a file 
for read access and the file exists in the container ("upperdir") and not in 
the image ("lowerdir"), it is read directly from the container.

- **The file exists in the container layer and the image layer**. If a 
container opens a file for read access and the file exists in the image layer 
and the container layer, the file's version in the container layer is read. 
This is because files in the container layer ("upperdir") obscure files with 
the same name in the image layer ("lowerdir").

Consider some scenarios where files in a container are modified.

- **Writing to a file for the first time**. The first time a container writes 
to an existing file, that file does not exist in the container ("upperdir"). 
The `overlay`/`overlay2` driver performs a *copy_up* operation to copy the file
from the image ("lowerdir") to the container ("upperdir"). The container then
writes the changes to the new copy of the file in the container layer.

    However, OverlayFS works at the file level not the block level. This means 
that all OverlayFS copy-up operations copy entire files, even if the file is 
very large and only a small part of it is being modified. This can have a 
noticeable impact on container write performance. However, two things are 
worth noting:

    * The copy_up operation only occurs the first time any given file is 
written to. Subsequent writes to the same file will operate against the copy of
 the file already copied up to the container.

    * OverlayFS only works with two layers. This means that performance should 
be better than AUFS which can suffer noticeable latencies when searching for 
files in images with many layers.

- **Deleting files and directories**. When files are deleted within a container
 a *whiteout* file is created in the containers "upperdir". The version of the 
file in the image layer ("lowerdir") is not deleted. However, the whiteout file
 in the container obscures it.

    Deleting a directory in a container results in *opaque directory* being 
created in the "upperdir". This has the same effect as a whiteout file and 
effectively masks the existence of the directory in the image's "lowerdir".

- **Renaming directories**. Calling `rename(2)` for a directory is allowed only 
when both of the source and the destination path are on the top layer. 
Otherwise, it returns `EXDEV` ("cross-device link not permitted").

So your application has to be designed so that it can handle `EXDEV` and fall 
back to a "copy and unlink" strategy.

## Configure Docker with the `overlay`/`overlay2` storage driver

To configure Docker to use the `overlay` storage driver your Docker host must be 
running version 3.18 of the Linux kernel (preferably newer) with the overlay 
kernel module loaded. For the `overlay2` driver, the version of your kernel must
be 4.0 or newer. OverlayFS can operate on top of most supported Linux filesystems.
However, ext4 is currently recommended for use in production environments.

The following procedure shows you how to configure your Docker host to use 
OverlayFS. The procedure assumes that the Docker daemon is in a stopped state.

> **Caution:** If you have already run the Docker daemon on your Docker host 
> and have images you want to keep, `push` them Docker Hub or your private 
> Docker Trusted Registry before attempting this procedure.

1. If it is running, stop the Docker `daemon`.

2. Verify your kernel version and that the overlay kernel module is loaded.

        $ uname -r

        3.19.0-21-generic

        $ lsmod | grep overlay

        overlay

3. Start the Docker daemon with the `overlay`/`overlay2` storage driver.

        $ dockerd --storage-driver=overlay &

        [1] 29403
        root@ip-10-0-0-174:/home/ubuntu# INFO[0000] Listening for HTTP on unix (/var/run/docker.sock)
        INFO[0000] Option DefaultDriver: bridge
        INFO[0000] Option DefaultNetwork: bridge
        <output truncated>

    Alternatively, you can force the Docker daemon to automatically start with
    the `overlay`/`overlay2` driver by editing the Docker config file and adding
    the `--storage-driver=overlay` flag to the `DOCKER_OPTS` line. Once this option
    is set you can start the daemon using normal startup scripts without having
    to manually pass in the `--storage-driver` flag.

4. Verify that the daemon is using the `overlay`/`overlay2` storage driver

        $ docker info

        Containers: 0
        Images: 0
        Storage Driver: overlay
         Backing Filesystem: extfs
        <output truncated>

    Notice that the *Backing filesystem* in the output above is showing as 
`extfs`. Multiple backing filesystems are supported but `extfs` (ext4) is 
recommended for production use cases.

Your Docker host is now using the `overlay`/`overlay2` storage driver. If you
run the `mount` command, you'll find Docker has automatically created the
`overlay` mount with the required "lowerdir", "upperdir", "merged" and "workdir"
constructs.

## OverlayFS and Docker Performance

As a general rule, the `overlay`/`overlay2` drivers should be fast. Almost
certainly faster than `aufs` and `devicemapper`. In certain circumstances it may
also be faster than `btrfs`. That said, there are a few things to be aware of
relative to the performance of Docker using the `overlay`/`overlay2` storage
drivers.

- **Page Caching**. OverlayFS supports page cache sharing. This means multiple
containers accessing the same file can share a single page cache entry (or
entries). This makes the `overlay`/`overlay2` drivers efficient with memory and
a good option for PaaS and other high density use cases.

- **copy_up**. As with AUFS, OverlayFS has to perform copy-up operations any 
time a container writes to a file for the first time. This can insert latency 
into the write operation &mdash; especially if the file being copied up is 
large. However, once the file has been copied up, all subsequent writes to that
 file occur without the need for further copy-up operations.

    The OverlayFS copy_up operation should be faster than the same operation 
with AUFS. This is because AUFS supports more layers than OverlayFS and it is 
possible to incur far larger latencies if searching through many AUFS layers.

- **Inode limits**. Use of the `overlay` storage driver can cause excessive 
inode consumption. This is especially so as the number of images and containers
 on the Docker host grows. A Docker host with a large number of images and lots
 of started and stopped containers can quickly run out of inodes. The `overlay2`
 does not have such an issue.

Unfortunately you can only specify the number of inodes in a filesystem at the 
time of creation. For this reason, you may wish to consider putting 
`/var/lib/docker` on a separate device with its own filesystem, or manually 
specifying the number of inodes when creating the filesystem.

The following generic performance best practices also apply to OverlayFS.

- **Solid State Devices (SSD)**. For best performance it is always a good idea 
to use fast storage media such as solid state devices (SSD).

- **Use Data Volumes**. Data volumes provide the best and most predictable 
performance. This is because they bypass the storage driver and do not incur 
any of the potential overheads introduced by thin provisioning and 
copy-on-write. For this reason, you should place heavy write workloads on data 
volumes.

## OverlayFS compatibility
To summarize the OverlayFS's aspect which is incompatible with other
filesystems:

- **open(2)**. OverlayFS only implements a subset of the POSIX standards. 
This can result in certain OverlayFS operations breaking POSIX standards. One 
such operation is the *copy-up* operation. Suppose that  your application calls 
`fd1=open("foo", O_RDONLY)` and then `fd2=open("foo", O_RDWR)`. In this case, 
your application expects `fd1` and `fd2` to refer to the same file. However, due 
to a copy-up operation that occurs after the first calling to `open(2)`, the 
descriptors refer to different files.

`yum` is known to be affected unless the `yum-plugin-ovl` package is installed. 
If the `yum-plugin-ovl` package is not available in your distribution (e.g. 
RHEL/CentOS prior to 6.8 or 7.2), you may need to run `touch /var/lib/rpm/*` 
before running `yum install`.

- **rename(2)**. OverlayFS does not fully support the `rename(2)` system call. 
Your application needs to detect its failure and fall back to a "copy and 
unlink" strategy.

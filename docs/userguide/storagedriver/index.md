<!--[metadata]>
+++
title = "Docker storage drivers"
description = "Learn how select the proper storage driver for your container."
keywords = ["container, storage, driver, AUFS, btfs, devicemapper,zvfs"]
[menu.main]
identifier = "mn_storage_docker"
parent = "mn_use_docker"
weight = 7
+++
<![end-metadata]-->


# Docker storage drivers

Docker relies on driver technology to manage the storage and interactions associated with images and they containers that run them. This section contains the following pages:

* [Understand images, containers, and storage drivers](imagesandcontainers.md)
* [Select a storage driver](selectadriver.md)
* [AUFS storage driver in practice](aufs-driver.md)
* [BTRFS storage driver in practice](btrfs-driver.md)
* [Device Mapper storage driver in practice](device-mapper-driver.md)
* [OverlayFS in practice](overlayfs-driver.md)
* [FS storage in practice](zfs-driver.md)

If you are new to Docker containers make sure you read ["Understand images, containers, and storage drivers"](imagesandcontainers.md) first. It explains key concepts and technologies that can help you when working with storage drivers.

### Acknowledgement

The Docker storage driver material was created in large part by our guest author
Nigel Poulton with a bit of help from Docker's own Jérôme Petazzoni. In his
spare time Nigel creates [IT training
videos](http://www.pluralsight.com/author/nigel-poulton), co-hosts the weekly
[In Tech We Trust podcast](http://intechwetrustpodcast.com/), and lives it large
on [Twitter](https://twitter.com/nigelpoulton).


&nbsp;

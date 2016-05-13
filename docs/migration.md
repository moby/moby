<!--[metadata]>
+++
title = "Migrate to Engine 1.10"
description = "Migrate to Engine 1.10"
keywords = ["docker, documentation, engine, upgrade, migration"]
[menu.main]
parent = "engine_use"
weight=79
+++
<![end-metadata]-->

# Migrate to Engine 1.10

Starting from version 1.10 of Docker Engine, we completely change the way image
data is addressed on disk. Previously, every image and layer used a randomly
assigned UUID. In 1.10 we implemented a content addressable method using an ID,
based on a secure hash of the image and layer data.

The new method gives users more security, provides a built-in way to avoid ID
collisions and guarantee data integrity after pull, push, load, or save. It also
brings better sharing of layers by allowing many images to freely share their
layers even if they didn’t come from the same build.

Addressing images by their content also lets us more easily detect if something
has already been downloaded. Because we have separated images and layers, you
don’t have to pull the configurations for every image that was part of the
original build chain. We also don’t need to create layers for the build
instructions that didn’t modify the filesystem.

Content addressability is the foundation for the new distribution features. The
image pull and push code has been reworked to use a download/upload manager
concept that makes pushing and pulling images much more stable and mitigates any
parallel request issues. The download manager also brings retries on failed
downloads and better prioritization for concurrent downloads.

We are also introducing a new manifest format that is built on top of the
content addressable base. It directly references the content addressable image
configuration and layer checksums. The new manifest format also makes it
possible for a manifest list to be used for targeting multiple
architectures/platforms. Moving to the new manifest format will be completely
transparent.

## Preparing for upgrade

To make your current images accessible to the new model we have to migrate them
to content addressable storage. This means calculating the secure checksums for
your current data.

All your current images, tags and containers are automatically migrated to the
new foundation the first time you start Docker Engine 1.10. Before loading your
container, the daemon will calculate all needed checksums for your current data,
and after it has completed, all your images and tags will have brand new secure
IDs.

**While this is simple operation, calculating SHA256 checksums for your files
can take time if you have lots of image data.** On average you should assume
that migrator can process data at a speed of 100MB/s. During this time your
Docker daemon won’t be ready to respond to requests.

## Minimizing migration time

If you can accept this one time hit, then upgrading Docker Engine and restarting
the daemon will transparently migrate your images. However, if you want to
minimize the daemon’s downtime, a migration utility can be run while your old
daemon is still running.

This tool will find all your current images and calculate the checksums for
them. After you upgrade and restart the daemon, the checksum data of the
migrated images will already exist, freeing the daemon from that computation
work. If new images appeared between the migration and the upgrade, those will
be processed at time of upgrade to 1.10.

[You can download the migration tool
here.](https://github.com/docker/v1.10-migrator/releases)

The migration tool can also be run as a Docker image. While running the migrator
image you need to expose your Docker data directory to the container. If you use
the default path then you would run:

    $ docker run --rm -v /var/lib/docker:/var/lib/docker docker/v1.10-migrator

If you use the
devicemapper storage driver, you also need to pass the flag `--privileged` to
give the tool access to your storage devices.

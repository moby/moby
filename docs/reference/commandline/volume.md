---
title: "Bind-mounts and Volumes"
description: "Bind-mounts and Volumes (--volume)"
keywords: "volume"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# Bind-mounts and Volumes

Docker supports two different kinds of mounts, which allow containers to read to
or write from files or directories on other containers or the host operating
system. These types are _data volumes_ (often referred to simply as volumes) and
_bind-mounts_.

Additionally, Docker also supports tmpfs mounts.

A **bind-mount** makes a file or directory on the host available to the
container it is mounted within. A bind-mount may be either read-only or
read-write. For example, a container might share its host's DNS information by
means of a bind-mount of the host's `/etc/resolv.conf` or a container might
write logs to its host's `/var/log/myContainerLogs` directory. If you use
bind-mounts and your host and containers have different notions of permissions,
access controls, or other such details, you will run into portability issues.

A **named volume** is a mechanism for decoupling persistent data needed by your
container from the image used to create the container and from the host machine.
Named volumes are created and managed by Docker, and a named volume persists
even when no container is currently using it. Data in named volumes can be
shared between a container and the host machine, as well as between multiple
containers. Docker uses a _volume driver_ to create, manage, and mount volumes.
You can back up or restore volumes using Docker commands.

A **tmpfs** mounts a tmpfs inside a container for volatile data.

Consider a situation where your image starts a lightweight web server. You could
use that image as a base image, copy in your website's HTML files, and package
that into another image. Each time your website changed, you'd need to update
the new image and redeploy all of the containers serving your website. A better
solution is to store the website in a named volume which is attached to each of
your web server containers when they start. To update the website, you just
update the named volume.

For more information about named volumes, see
[Data Volumes](https://docs.docker.com/engine/tutorials/dockervolumes/).

## The short-syntax

The short-syntax (e.g. `--volume my-volume:/path/in/container`) is the
traditional syntax and only supported in `docker run` and `docker create`.

The usage is described in [`docker run`](run.md).

## The long-syntax

The long-syntax (e.g. `--volume type=volume,source=my-volume,destination=/path/in/container`)
is the newer and more flexible syntax originally designed for `docker service`.
Since v1.14, the long-syntax is also available fo `docker run` and `docker create`
as well.

The following table describes options which apply to both bind-mounts and named
volumes in the long-syntax

| Option                                   | Required                  | Description
|:-----------------------------------------|:--------------------------|:-----------------------------------------------------------------------------------------
| **type**                                 |                           | The type of mount, can be either `volume`, `bind`, or `tmpfs`. Defaults to `volume` if no type is specified.<ul><li>`volume`: mounts a [managed volume](volume_create.md) into the container.</li><li>`bind`: bind-mounts a directory or file from the host into the container.</li><li>`tmpfs`: mount a tmpfs in the container</li></ul>
| **src** or **source**                    | for `type=bind`&nbsp;only | <ul><li>`type=volume`: `src` is an optional way to specify the name of the volume (for example, `src=my-volume`). If the named volume does not exist, it is automatically created. If no `src` is specified, the volume is assigned a random name which is guaranteed to be unique on the host, but may not be unique cluster-wide. A randomly-named volume has the same lifecycle as its container and is destroyed when the *container* is destroyed (which is upon `service update`, or when scaling or re-balancing the service).</li><li>`type=bind`: `src` is required, and specifies an absolute path to the file or directory to bind-mount (for example, `src=/path/on/host/`).  An error is produced if the file or directory does not exist.</li><li>`type=tmpfs`: `src` is not supported.</li></ul>
| **dst** or **destination** or **target** | yes                       | Mount path inside the container, for example `/some/path/in/container/`. If the path does not exist in the container's filesystem, the Engine creates a directory at the specified location before mounting the volume or bind-mount.
| **readonly** or **ro**                   |                           | The Engine mounts binds and volumes `read-write` unless `readonly` option is given when mounting the bind or volume.<br /><br /><ul><li>`true` or `1` or no value: Mounts the bind or volume read-only.</li><li>`false` or `0`: Mounts the bind or volume read-write.</li></ul>

### Bind Propagation

Bind propagation refers to whether or not mounts created within a given
bind-mount or named volume can be propagated to replicas of that mount. Consider
a mount point `/mnt`, which is also mounted on `/tmp`. The propation settings
control whether a mount on `/tmp/a` would also be available on `/mnt/a`. Each
propagation setting has a recursive counterpoint. In the case of recursion,
consider that `/tmp/a` is also mounted as `/foo`. The propagation settings
control whether `/mnt/a` and/or `/tmp/a` would exist.

The `bind-propagation` option defaults to `rprivate` for both bind-mounts and
volume mounts, and is only configurable for bind-mounts. In other words, named
volumes do not support bind propagation.

- **`shared`**: Sub-mounts of the original mount are exposed to replica mounts,
                and sub-mounts of replica mounts are also propagated to the
                original mount.
- **`slave`**: similar to a shared mount, but only in one direction. If the
               original mount exposes a sub-mount, the replica mount can see it.
               However, if the replica mount exposes a sub-mount, the original
               mount cannot see it.
- **`private`**: The mount is private. Sub-mounts within it are not exposed to
                 replica mounts, and sub-mounts of replica mounts are not
                 exposed to the original mount.
- **`rshared`**: The same as shared, but the propagation also extends to and from
                 mount points nested within any of the original or replica mount
                 points.
- **`rslave`**: The same as `slave`, but the propagation also extends to and from
                 mount points nested within any of the original or replica mount
                 points.
- **`rprivate`**: The default. The same as `private`, meaning that no mount points
                  anywhere within the original or replica mount points propagate
                  in either direction.

For more information about bind propagation, see the
[Linux kernel documentation for shared subtree](https://www.kernel.org/doc/Documentation/filesystems/sharedsubtree.txt).

### Options for Named Volumes
The following options can only be used for named volumes (`type=volume`);

| Option                | Description
|:----------------------|:--------------------------------------------------------------------------------------------------------------------
| **volume-driver**     | Name of the volume-driver plugin to use for the volume. Defaults to ``"local"``, to use the local volume driver to create the volume if the volume does not exist.
| **volume-label**      | One or more custom metadata ("labels") to apply to the volume upon creation. For example, `volume-label=mylabel=hello-world,my-other-label=hello-mars`. For more information about labels, refer to [apply custom metadata](https://docs.docker.com/engine/userguide/labels-custom-metadata/).
| **volume-nocopy**     | By default, if you attach an empty volume to a container, and files or directories already existed at the mount-path in the container (`dst`), the Engine copies those files and directories into the volume, allowing the host to access them. Set `volume-nocopy` to disables copying files from the container's filesystem to the volume and mount the empty volume.<br /><br />A value is optional:<ul><li>`true` or `1`: Default if you do not provide a value. Disables copying.</li><li>`false` or `0`: Enables copying.</li></ul>
| **volume-opt**        | Options specific to a given volume driver, which will be passed to the driver when creating the volume. Options are provided as a comma-separated list of key/value pairs, for example, `volume-opt=some-option=some-value,some-other-option=some-other-value`. For available options for a given driver, refer to that driver's documentation.

### Options for tmpfs
The following options can only be used for tmpfs mounts (`type=tmpfs`);

| Option                | Description
|:----------------------|:--------------------------------------------------------------------------------------------------------------------
| **tmpfs-size**        | Size of the tmpfs mount in bytes. Unlimited by default in Linux.
| **tmpfs-mode**        | File mode of the tmpfs in octal. (e.g. `"700"` or `"0700"`.) Defaults to ``"1777"`` in Linux.

## Differences between the long-syntax and the short-syntax

The long-syntax supports most options that are supported by the traditional
short-syntax, with some important exceptions:

- The long-syntax allows you to specify a volume driver and volume driver
    options *per volume*, without creating the volumes in advance. In contrast,
    the short-syntax allows you to specify a single volume driver which is shared
    by all volumes, using the `--volume-driver` flag.

- The long-syntax flag allows you to specify custom metadata ("labels") for a volume,
    before the volume is created.

- When you use the long-syntax with `type=bind`, the host-path must refer to an *existing*
    path on the host. The path will not be created for you and the service will fail
    with an error if the path does not exist.

- The long-syntax does not allow you to relabel a volume with `Z` or `z` flags,
    which are used for `selinux` labeling.

- The short-syntax is not available for `docker service`.

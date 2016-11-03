---
title: "volume create"
description: "The volume create command description and usage"
keywords: "volume, create"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# volume create

```markdown
Usage:  docker volume create [OPTIONS] [VOLUME]

Create a volume

Options:
  -d, --driver string   Specify volume driver name (default "local")
      --help            Print usage
      --label value     Set metadata for a volume (default [])
  -o, --opt value       Set driver specific options (default map[])
```

Creates a new volume that containers can consume and store data in. If a name is not specified, Docker generates a random name. You create a volume and then configure the container to use it, for example:

```bash
$ docker volume create hello
hello

$ docker run -d -v hello:/world busybox ls /world
```

The mount is created inside the container's `/world` directory. Docker does not support relative paths for mount points inside the container.

Multiple containers can use the same volume in the same time period. This is useful if two containers need access to shared data. For example, if one container writes and the other reads the data.

Volume names must be unique among drivers.  This means you cannot use the same volume name with two different drivers.  If you attempt this `docker` returns an error:

```
A volume named  "hello"  already exists with the "some-other" driver. Choose a different volume name.
```

If you specify a volume name already in use on the current driver, Docker assumes you want to re-use the existing volume and does not return an error.   

## Driver specific options

Some volume drivers may take options to customize the volume creation. Use the `-o` or `--opt` flags to pass driver options:

```bash
$ docker volume create --driver fake --opt tardis=blue --opt timey=wimey
```

These options are passed directly to the volume driver. Options for
different volume drivers may do different things (or nothing at all).

The built-in `local` driver on Windows does not support any options.

The built-in `local` driver on Linux accepts options similar to the linux `mount` command. You can provide multiple options by passing the `--opt` flag multiple times. Some `mount` options (such as the `o` option) can take a comma-separated list of options. Complete list of available mount options can be found [here](http://man7.org/linux/man-pages/man8/mount.8.html).

For example, the following creates a `tmpfs` volume called `foo` with a size of 100 megabyte and `uid` of 1000.

```bash
$ docker volume create --driver local --opt type=tmpfs --opt device=tmpfs --opt o=size=100m,uid=1000 foo
```

Another example that uses `btrfs`:

```bash
$ docker volume create --driver local --opt type=btrfs --opt device=/dev/sda2 foo
```

Another example that uses `nfs` to mount the `/path/to/dir` in `rw` mode from `192.168.1.1`:

```bash
$ docker volume create --driver local --opt type=nfs --opt o=addr=192.168.1.1,rw --opt device=:/path/to/dir foo
```


## Related information

* [volume inspect](volume_inspect.md)
* [volume ls](volume_ls.md)
* [volume rm](volume_rm.md)
* [volume prune](volume_prune.md)
* [Understand Data Volumes](https://docs.docker.com/engine/tutorials/dockervolumes/)

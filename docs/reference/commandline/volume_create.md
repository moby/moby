<!--[metadata]>
+++
title = "volume create"
description = "The volume create command description and usage"
keywords = ["volume, create"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# volume create

    Usage: docker volume create [OPTIONS]

    Create a volume

      -d, --driver=local    Specify volume driver name
      --help=false          Print usage
      --name=               Specify volume name
      -o, --opt=map[]       Set driver specific options

Creates a new volume that containers can consume and store data in. If a name is not specified, Docker generates a random name. You create a volume and then configure the container to use it, for example:

    $ docker volume create --name hello
    hello

    $ docker run -d -v hello:/world busybox ls /world

The mount is created inside the container's `/world` directory. Docker does not support relative paths for mount points inside the container.

Multiple containers can use the same volume in the same time period. This is useful if two containers need access to shared data. For example, if one container writes and the other reads the data.

Volume names must be unique among drivers.  This means you cannot use the same volume name with two different drivers.  If you attempt this `docker` returns an error:

```
A volume named  "hello"  already exists with the "some-other" driver. Choose a different volume name.
```

If you specify a volume name already in use on the current driver, Docker assumes you want to re-use the existing volume and does not return an error.   

## Driver specific options

Some volume drivers may take options to customize the volume creation. Use the `-o` or `--opt` flags to pass driver options:

    $ docker volume create --driver fake --opt tardis=blue --opt timey=wimey

These options are passed directly to the volume driver. Options for
different volume drivers may do different things (or nothing at all).

*Note*: The built-in `local` volume driver does not currently accept any options.

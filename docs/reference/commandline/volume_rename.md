<!--[metadata]>
+++
title = "volume rename"
description = "the volume rename command description and usage"
keywords = ["volume, rename"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# volume rename

    Usage: docker volume rename [OPTIONS] OLD_NAME NEW_NAME

    Rename a volume

      --help=false       Print usage

The `docker volume rename` command allows the volume to be renamed to a different name.
    $ docker volume create --name volume1
    volume1
    $ docker volume ls
    DRIVER              VOLUME NAME
    local               volume1
    $ docker volume rename volume1 volume2

    $ docker volume ls
    DRIVER              VOLUME NAME
    local               volume2


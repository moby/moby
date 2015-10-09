<!--[metadata]>
+++
title = "export"
description = "The export command description and usage"
keywords = ["export, file, system, container"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# export

    Usage: docker export [OPTIONS] CONTAINER

    Export the contents of a container's filesystem as a tar archive

      --help=false       Print usage
      -o, --output=""    Write to a file, instead of STDOUT

The `docker export` command does not export the contents of volumes associated
with the container. If a volume is mounted on top of an existing directory in
the container, `docker export` will export the contents of the *underlying*
directory, not the contents of the volume.

Refer to [Backup, restore, or migrate data
volumes](../../userguide/dockervolumes.md#backup-restore-or-migrate-data-volumes) in
the user guide for examples on exporting data in a volume.

## Examples

    $ docker export red_panda > latest.tar

Or

    $ docker export --output="latest.tar" red_panda

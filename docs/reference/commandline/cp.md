<!--[metadata]>
+++
title = "cp"
description = "The cp command description and usage"
keywords = ["copy, container, files, folders"]
[menu.main]
parent = "smn_cli"
weight=1
+++
<![end-metadata]-->

# cp

    Usage: docker cp CONTAINER:PATH HOSTDIR|-

    Copy files/folders from the PATH to the HOSTDIR.

Copy files or folders from a container's filesystem to the directory on the
host. Use '-' to write the data as a tar file to `STDOUT`. `CONTAINER:PATH` is
relative to the root of the container's filesystem.



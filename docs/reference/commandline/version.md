<!--[metadata]>
+++
title = "version"
description = "The version command description and usage"
keywords = ["version, architecture, api"]
[menu.main]
parent = "smn_cli"
weight=1
+++
<![end-metadata]-->

# version

    Usage: docker version

    Show the Docker version information.

Show the Docker version, API version, Go version, Git commit, Build date/time,
and OS/architecture of both Docker client and daemon. Example use:

    $ docker version
	Client:
	 Version:      1.8.0
	 API version:  1.20
	 Go version:   go1.4.2
	 Git commit:   f5bae0a
	 Built:        Tue Jun 23 17:56:00 UTC 2015
	 OS/Arch:      linux/amd64

	Server:
	 Version:      1.8.0
	 API version:  1.20
	 Go version:   go1.4.2
	 Git commit:   f5bae0a
	 Built:        Tue Jun 23 17:56:00 UTC 2015
	 OS/Arch:      linux/amd64
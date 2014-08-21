page_title: Installation on Gentoo
page_description: Installation instructions for Docker on Gentoo.
page_keywords: gentoo linux, virtualization, docker, documentation, installation

# Gentoo

Installing Docker on Gentoo Linux can be accomplished using one of two
methods. The first and best way if you're looking for a stable
experience is to use the official app-emulation/docker package directly
in the portage tree.

If you're looking for a `-bin` ebuild, a live ebuild, or bleeding edge
ebuild changes/fixes, the second installation method is to use the
overlay provided at
[https://github.com/tianon/docker-overlay](https://github.com/tianon/docker-overlay)
which can be added using `app-portage/layman`. The most accurate and
up-to-date documentation for properly installing and using the overlay
can be found in [the overlay
README](https://github.com/tianon/docker-overlay/blob/master/README.md#using-this-overlay).

Note that sometimes there is a disparity between the latest version and
what's in the overlay, and between the latest version in the overlay and
what's in the portage tree. Please be patient, and the latest version
should propagate shortly.

## Installation

The package should properly pull in all the necessary dependencies and
prompt for all necessary kernel options. The ebuilds for 0.7+ include
use flags to pull in the proper dependencies of the major storage
drivers, with the "device-mapper" use flag being enabled by default,
since that is the simplest installation path.

    $ sudo emerge -av app-emulation/docker

If any issues arise from this ebuild or the resulting binary, including
and especially missing kernel configuration flags and/or dependencies,
[open an issue on the docker-overlay repository](
https://github.com/tianon/docker-overlay/issues) or ping
tianon directly in the #docker IRC channel on the freenode network.

Other use flags are described in detail on [tianon's
blog](https://tianon.github.io/post/2014/05/17/docker-on-gentoo.html).

## Starting Docker

Ensure that you are running a kernel that includes all the necessary
modules and/or configuration for LXC (and optionally for device-mapper
and/or AUFS, depending on the storage driver you`ve decided to use).

### OpenRC

To start the docker daemon:

    $ sudo /etc/init.d/docker start

To start on system boot:

    $ sudo rc-update add docker default

### systemd

To start the docker daemon:

    $ sudo systemctl start docker.service

To start on system boot:

    $ sudo systemctl enable docker.service

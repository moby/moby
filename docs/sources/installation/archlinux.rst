.. _arch_linux:

Arch Linux
==========

Installing on Arch Linux is not officially supported but can be handled via 
either of the following AUR packages:

* `dotcloud-docker <https://aur.archlinux.org/packages/dotcloud-docker/>`_
* `dotcloud-docker-git <https://aur.archlinux.org/packages/dotcloud-docker-git/>`_

The dotcloud-docker package will install the latest tagged version of docker. 
The dotcloud-docker-git package will build from the current master branch.

Dependencies
------------

Docker depends on several packages which will be installed automatically with
either AUR package.

* aufs3
* bridge-utils
* go
* iproute2
* linux-aufs_friendly

Installation
------------

The instructions here assume **yaourt** is installed.  See 
`Arch User Repository <https://wiki.archlinux.org/index.php/Arch_User_Repository#Installing_packages>`_
for information on building and installing packages from the AUR if you have not
done so before.

Keep in mind that if **linux-aufs_friendly** is not already installed that a
new kernel will be compiled and this can take quite a while.

::

    yaourt -S dotcloud-docker-git

Prior to starting docker modify your bootloader to use the 
**linux-aufs_friendly** kernel and reboot your system.

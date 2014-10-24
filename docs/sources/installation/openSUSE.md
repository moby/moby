page_title: Installation on openSUSE
page_description: Installation instructions for Docker on openSUSE.
page_keywords: openSUSE, virtualbox, docker, documentation, installation

# openSUSE

Docker is available in **openSUSE 12.3 and later**. Please note that due
to the current Docker limitations Docker is able to run only on the **64
bit** architecture.

## Installation

The `docker` package from the [Virtualization
project](https://build.opensuse.org/project/show/Virtualization) on
[OBS](https://build.opensuse.org/) provides Docker on openSUSE.

To proceed with Docker installation please add the right Virtualization
repository.

    # openSUSE 12.3
    $ sudo zypper ar -f http://download.opensuse.org/repositories/Virtualization/openSUSE_12.3/ Virtualization
    $ sudo rpm --import http://download.opensuse.org/repositories/Virtualization/openSUSE_12.3/repodata/repomd.xml.key

    # openSUSE 13.1
    $ sudo zypper ar -f http://download.opensuse.org/repositories/Virtualization/openSUSE_13.1/ Virtualization
    $ sudo rpm --import http://download.opensuse.org/repositories/Virtualization/openSUSE_13.1/repodata/repomd.xml.key

Install the Docker package.

    $ sudo zypper in docker

It's also possible to install Docker using openSUSE's1-click install.
Just visit [this](http://software.opensuse.org/package/docker) page,
select your openSUSE version and click on the installation link. This
will add the right repository to your system and it will also install
the docker package.

Now that it's installed, let's start the Docker daemon.

    $ sudo systemctl start docker

If we want Docker to start at boot, we should also:

    $ sudo systemctl enable docker

The docker package creates a new group named docker. Users, other than
root user, need to be part of this group in order to interact with the
Docker daemon. You can add users with:

    $ sudo /usr/sbin/usermod -a -G docker <username>

To verify that everything has worked as expected:

    $ sudo docker run --rm -i -t opensuse /bin/bash

This should download and import the `opensuse` image, and then start `bash` in
a container. To exit the container type `exit`.

If you want your containers to be able to access the external network you must
enable the `net.ipv4.ip_forward` rule.
This can be done using YaST by browsing to the
`Network Devices -> Network Settings -> Routing` menu and ensuring that the
`Enable IPv4 Forwarding` box is checked.

This option cannot be changed when networking is handled by the Network Manager.
In such cases the `/etc/sysconfig/SuSEfirewall2` file needs to be edited by
hand to ensure the `FW_ROUTE` flag is set to `yes` like so:

    FW_ROUTE="yes"


**Done!**

Continue with the [User Guide](/userguide/).


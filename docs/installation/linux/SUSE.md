<!--[metadata]>
+++
aliases = [ "/engine/installation/SUSE/"]
title = "Installation on openSUSE and SUSE Linux Enterprise"
description = "Installation instructions for Docker on openSUSE and on SUSE Linux Enterprise."
keywords = ["openSUSE, SUSE Linux Enterprise, SUSE, SLE, docker, documentation,  installation"]
[menu.main]
parent = "engine_linux"
+++
<![end-metadata]-->

# openSUSE and SUSE Linux Enterprise

This page provides instructions for installing and configuring the latest
Docker Engine software on openSUSE and SUSE systems.

>**Note:** You can also find bleeding edge Docker versions inside of the repositories maintained by the [Virtualization:containers project](https://build.opensuse.org/project/show/Virtualization:containers) on the [Open Build Service](https://build.opensuse.org/). This project delivers also other packages that are related with the Docker ecosystem (for example, Docker Compose).

## Prerequisites

You must be running a 64 bit architecture.

## Add Repositories

#### openSUSE

Docker is part of the official openSUSE repositories starting from 13.2. No
additional repository is required on your system.

#### SUSE Linux Enterprise

Docker is officially supported on SUSE Linux Enterprise 12 and later. You can find the latest supported Docker packages inside the `Container` module. To enable this module, do the following:

1. Start YaST, and select *Software > Software Repositories*.
2. Click *Add* to open the add-on dialog.
3. Select *Extensions and Module from Registration Server* and click *Next*.
4. From the list of available extensions and modules, select *Container Module* and click *Next*.
   The containers module and its repositories are added to your system.
5. If you use Subscription Management Tool, update the list of repositories at the SMT server.

Otherwise execute the following command:

    # SUSEConnect -p sle-module-containers/12/x86_64 -r ''

    >**Note:** currently the `-r ''` flag is required to avoid a known limitation of `SUSEConnect`.

The [Virtualization:containers project](https://build.opensuse.org/project/show/Virtualization:containers)
on the [Open Build Service](https://build.opensuse.org/) contains also bleeding
edge Docker packages for SUSE Linux Enterprise. However these packages are
**not supported** by SUSE.

## Install Docker
The following should be executed in a root console

1. Install the Docker package:

        # zypper in docker

2. Start the Docker daemon.

        # systemctl start docker

3. Test the Docker installation(optional).

        # docker run hello-world

## Configure Docker boot options

After installation, by default docker still needs to be started manually each time you wish to use docker. To start the `docker daemon` at boot(advisable and convenient), set the following:

    # systemctl enable docker

## Elevating an ordinary User for administering Docker

This section describes elevating a normal User account with near-root permissions which can lead to your network and perhaps multiple machines in your network being "owned." Consider the implications of this in your situation before proceeding.

When the docker package is installed, a new group named `docker` is created. When ordinary User accounts are added to this group, they will have full docker administrative rights so can execute docker commands, including creating, managing, modifying and deleting. 

You can add users to be granted docker administrative rights either through YAST or with the following command:

    # /usr/sbin/usermod -a -G docker <username>

Once you add a user, the User must log out and back in to pick up these new permissions.

## Enable external network access

Configuring a container's **outbound** networking is simple and typically only requires attaching to a network object with external network properties.

For instance:

    # docker run -it opensuse net=host /bin/bash

Configuring **inbound** access might be as easy as sharing the Host's network interface and configuring an unused port, or if your container is to have its own network interface, then you must also configure IP Forwarding on the Host to your container's interface.

Configuring this `net.ipv4.ip_forward` rule is best accomplished using YaST as follows...

**If using Wicked to manage your networking:**</br>
For openSUSE Tumbleweed and LEAP, open YAST and browse to **System -> Network Settings -> Routing**. 

For SUSE Linux Enterprise 12, openSUSE 13.2 and earlier, browse to **Network Devices -> Network Settings -> Routing** menu (f) and check the *Enable IPv4 Forwarding* box.

**If using Network Manager to manage your networking:**</br>
You should edit the `/etc/sysconfig/SuSEfirewall2` file to include the following line:

    FW_ROUTE="yes"

##### For more container networking:

Docker Documentation for configuring container networks on a single Host</br>
https://docs.docker.com/engine/userguide/networking/dockernetworks/</br>
Docker documentation for configuring container networks that span multiple Hosts</br>
https://docs.docker.com/engine/userguide/networking/get-started-overlay

## Custom docker daemon options

Common options that involve modifying the docker daemon

* HTTP Proxy
* Non-default location for runtime files
* Alternate, non-default storage for images and container files
* More

Read the systemd article to
learn how to [customize your systemd Docker daemon options](../../admin/systemd.md).

## Uninstallation

To uninstall the Docker package:

    # zypper rm docker

The above command does not remove images, containers, volumes, or user created
configuration files on your host. If your User-created assets are still stored in their default locations (see "docker daemon options" above which can change) and you wish to delete them all,  run the following command:

    # rm -rf /var/lib/docker

You must delete the user created configuration files manually.

## Where to go from here

You can find more details about Docker on openSUSE or SUSE Linux Enterprise in the
[Docker quick start guide](https://www.suse.com/documentation/sles-12/dockerquick/data/dockerquick.html)
on the SUSE website. The document targets SUSE Linux Enterprise, but its contents apply also to openSUSE.</br>
Various Community articles on Docker exist as well. One volunteer's collection can be found here</br>
https://en.opensuse.org/User:Tsu2#Docker


Continue to the [User Guide](../../userguide/index.md).

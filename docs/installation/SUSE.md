<!--[metadata]>
+++
title = "Installation on openSUSE and SUSE Linux Enterprise"
description = "Installation instructions for Docker on openSUSE and on SUSE Linux Enterprise."
keywords = ["openSUSE, SUSE Linux Enterprise, SUSE, SLE, docker, documentation,  installation"]
[menu.main]
parent = "smn_linux"
+++
<![end-metadata]-->

# openSUSE and SUSE Linux Enterprise

This page provides instructions for installing and configuring the lastest
Docker Engine software on openSUSE and SUSE systems.

>**Note:** You can also find bleeding edge Docker versions inside of the repositories maintained by the [Virtualization:containers project](https://build.opensuse.org/project/show/Virtualization:containers) on the [Open Build Service](https://build.opensuse.org/). This project delivers also other packages that are related with the Docker ecosystem (for example, Docker Compose).

## Prerequisites

You must be running a 64 bit architecture.

## openSUSE

Docker is part of the official openSUSE repositories starting from 13.2. No
additional repository is required on your system.

## SUSE Linux Enterprise

Docker is officially supported on SUSE Linux Enterprise 12 and later. You can find the latest supported Docker packages inside the `Container` module. To enable this module, do the following:

1. Start YaST, and select *Software > Software Repositories*.
2. Click *Add* to open the add-on dialog.
3. Select *Extensions and Module from Registration Server* and click *Next*.
4. From the list of available extensions and modules, select *Container Module* and click *Next*.
   The containers module and its repositories are added to your system.
5. If you use Subscription Management Tool, update the list of repositories at the SMT server.

Otherwise execute the following command:

    $ sudo SUSEConnect -p sle-module-containers/12/x86_64 -r ''

    >**Note:** currently the `-r ''` flag is required to avoid a known limitation of `SUSEConnect`.

The [Virtualization:containers project](https://build.opensuse.org/project/show/Virtualization:containers)
on the [Open Build Service](https://build.opensuse.org/) contains also bleeding
edge Docker packages for SUSE Linux Enterprise. However these packages are
**not supported** by SUSE.

### Install Docker

1. Install the Docker package:

        $ sudo zypper in docker

2. Start the Docker daemon.

        $ sudo systemctl start docker

3. Test the Docker installation.

        $ sudo docker run hello-world

## Configure Docker boot options

You can use these steps on openSUSE or SUSE Linux Enterprise. To start the `docker daemon` at boot, set the following:

    $ sudo systemctl enable docker

The `docker` package creates a new group named `docker`. Users, other than
`root` user, must be part of this group to interact with the
Docker daemon. You can add users with this command syntax:

    sudo /usr/sbin/usermod -a -G docker <username>

Once you add a user, make sure they relog to pick up these new permissions.

## Enable external network access

If you want your containers to be able to access the external network, you must
enable the `net.ipv4.ip_forward` rule. To do this, use YaST.

For openSUSE Tumbleweed and later, browse to the **System -> Network Settings -> Routing** menu. For SUSE Linux Enterprise 12 and previous openSUSE versions, browse to **Network Devices -> Network Settings -> Routing** menu (f) and check the *Enable IPv4 Forwarding* box.

When networking is handled by the Network Manager, instead of YaST you must edit
the `/etc/sysconfig/SuSEfirewall2` file needs by hand to ensure the `FW_ROUTE`
flag is set to `yes` like so:

    FW_ROUTE="yes"

## Custom daemon options

If you need to add an HTTP Proxy, set a different directory or partition for the
Docker runtime files, or make other customizations, read the systemd article to
learn how to [customize your systemd Docker daemon options](../articles/systemd.md).

## Uninstallation

To uninstall the Docker package:

    $ sudo zypper rm docker

The above command does not remove images, containers, volumes, or user created
configuration files on your host. If you wish to delete all images, containers,
and volumes run the following command:

    $ rm -rf /var/lib/docker

You must delete the user created configuration files manually.

## Where to go from here

You can find more details about Docker on openSUSE or SUSE Linux Enterprise in
the [Docker quick start guide](https://www.suse.com/documentation/sles-12/dockerquick/data/dockerquick.
html) on the SUSE website. The document targets SUSE Linux Enterprise, but its contents apply also to openSUSE.

Continue to the [User Guide](../userguide/).

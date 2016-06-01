<!--[metadata]>
+++
aliases = [ "/engine/installation/oracle/"]
title = "Installation on Oracle Linux"
description = "Installation instructions for Docker on Oracle Linux."
keywords = ["Docker, Docker documentation, requirements, linux, rhel, centos, oracle,  ol"]
[menu.main]
parent = "engine_linux"
+++
<![end-metadata]-->

# Oracle Linux

Docker is supported Oracle Linux 6 and 7. You do not require an Oracle Linux
Support subscription to install Docker on Oracle Linux.

## Prerequisites

Due to current Docker limitations, Docker is only able to run only on the x86_64
architecture. Docker requires the use of the Unbreakable Enterprise Kernel
Release 4 (4.1.12) or higher on Oracle Linux. This kernel supports the Docker
btrfs storage engine on both Oracle Linux 6 and 7.

## Install


> **Note**: The procedure below installs binaries built by Docker. These binaries
> are not covered by Oracle Linux support. To ensure Oracle Linux support, please
> follow the installation instructions provided in the
> [Oracle Linux documentation](https://docs.oracle.com/en/operating-systems/?tab=2).
>
> The installation instructions for Oracle Linux 6 and 7 can be found in [Chapter 2 of
> the Docker User&apos;s Guide](https://docs.oracle.com/cd/E52668_01/E75728/html/docker_install_upgrade.html)


1. Log into your machine as a user with `sudo` or `root` privileges.

2. Make sure your existing yum packages are up-to-date.

        $ sudo yum update

3. Add the yum repo yourself.

    For version 6:

        $ sudo tee /etc/yum.repos.d/docker.repo <<-EOF
        [dockerrepo]
        name=Docker Repository
        baseurl=https://yum.dockerproject.org/repo/main/oraclelinux/6
        enabled=1
        gpgcheck=1
        gpgkey=https://yum.dockerproject.org/gpg
        EOF

    For version 7:

        $ cat >/etc/yum.repos.d/docker.repo <<-EOF
        [dockerrepo]
        name=Docker Repository
        baseurl=https://yum.dockerproject.org/repo/main/oraclelinux/7
        enabled=1
        gpgcheck=1
        gpgkey=https://yum.dockerproject.org/gpg
        EOF

4. Install the Docker package.

        $ sudo yum install docker-engine

5. Start the Docker daemon.

     On Oracle Linux 6:

        $ sudo service docker start

     On Oracle Linux 7:

        $ sudo systemctl start docker.service

6. Verify `docker` is installed correctly by running a test image in a container.

        $ sudo docker run hello-world

## Optional configurations

This section contains optional procedures for configuring your Oracle Linux to work
better with Docker.

* [Create a docker group](#create-a-docker-group)
* [Configure Docker to start on boot](#configure-docker-to-start-on-boot)
* [Use the btrfs storage engine](#use-the-btrfs-storage-engine)

### Create a Docker group		

The `docker` daemon binds to a Unix socket instead of a TCP port. By default
that Unix socket is owned by the user `root` and other users can access it with
`sudo`. For this reason, `docker` daemon always runs as the `root` user.

To avoid having to use `sudo` when you use the `docker` command, create a Unix
group called `docker` and add users to it. When the `docker` daemon starts, it
makes the ownership of the Unix socket read/writable by the `docker` group.

>**Warning**: The `docker` group is equivalent to the `root` user; For details
>on how this impacts security in your system, see [*Docker Daemon Attack
>Surface*](../../security/security.md#docker-daemon-attack-surface) for details.

To create the `docker` group and add your user:

1. Log into Oracle Linux as a user with `sudo` privileges.

2. Create the `docker` group.

        sudo groupadd docker

3. Add your user to `docker` group.

        sudo usermod -aG docker username

4. Log out and log back in.

    This ensures your user is running with the correct permissions.

5. Verify your work by running `docker` without `sudo`.

        $ docker run hello-world

	If this fails with a message similar to this:

		Cannot connect to the Docker daemon. Is 'docker daemon' running on this host?

	Check that the `DOCKER_HOST` environment variable is not set for your shell.
	If it is, unset it.

### Configure Docker to start on boot

You can configure the  Docker daemon to start automatically at boot.

On Oracle Linux 6:

```
$ sudo chkconfig docker on
```

On Oracle Linux 7:

```
$ sudo systemctl enable docker.service
```

If you need to add an HTTP Proxy, set a different directory or partition for the
Docker runtime files, or make other customizations, read our systemd article to
learn how to [customize your systemd Docker daemon options](../../admin/systemd.md).

### Use the btrfs storage engine

Docker on Oracle Linux 6 and 7 supports the use of the btrfs storage engine.
Before enabling btrfs support, ensure that `/var/lib/docker` is stored on a
btrfs-based filesystem. Review [Chapter
5](http://docs.oracle.com/cd/E37670_01/E37355/html/ol_btrfs.html) of the [Oracle
Linux Administrator's Solution
Guide](http://docs.oracle.com/cd/E37670_01/E37355/html/index.html) for details
on how to create and mount btrfs filesystems.

To enable btrfs support on Oracle Linux:

1. Ensure that `/var/lib/docker` is on a btrfs filesystem.

2. Edit `/etc/sysconfig/docker` and add `-s btrfs` to the `OTHER_ARGS` field.

3. Restart the Docker daemon:

## Uninstallation

To uninstall the Docker package:

    $ sudo yum -y remove docker-engine

The above command will not remove images, containers, volumes, or user created
configuration files on your host. If you wish to delete all images, containers,
and volumes run the following command:

    $ rm -rf /var/lib/docker

You must delete the user created configuration files manually.

## Known issues

### Docker unmounts btrfs filesystem on shutdown
If you're running Docker using the btrfs storage engine and you stop the Docker
service, it will unmount the btrfs filesystem during the shutdown process. You
should ensure the filesystem is mounted properly prior to restarting the Docker
service.

On Oracle Linux 7, you can use a `systemd.mount` definition and modify the
Docker `systemd.service` to depend on the btrfs mount defined in systemd.

### SElinux support on Oracle Linux 7
SElinux must be set to `Permissive` or `Disabled` in `/etc/sysconfig/selinux` to
use the btrfs storage engine on Oracle Linux 7.

## Further issues?

If you have a current Basic or Premier Support Subscription for Oracle Linux,
you can report any issues you have with the installation of Docker via a Service
Request at [My Oracle Support](http://support.oracle.com).

If you do not have an Oracle Linux Support Subscription, you can use the [Oracle
Linux
Forum](https://community.oracle.com/community/server_%26_storage_systems/linux/oracle_linux) for community-based support.

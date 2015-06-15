page_title: Installation on Debian
page_description: Instructions for installing Docker on Debian.
page_keywords: Docker, Docker documentation, installation, debian

# Debian

Docker is supported on the following versions of Debian:

 - [*Debian 8.0 Jessie (64-bit)*](#debian-jessie-80-64-bit)
 - [*Debian 7.0 Wheezy (64-bit)*](#debian-wheezystable-7x-64-bit)

> **Note**:
> Debian 8 Jessie contains a much older KDE3/GNOME2 package called ``docker``,
> so do not mistake this for the docker client or daemon.

### Installation

The docker package can be installed using the online installer with `curl`. To
make sure that you have `curl` installed run the following as root:

    # apt-get install curl

The docker package can be installed on `Debian 8 Jessie` using the docker
install tool. Run the following as root user:

    # curl -sSL https://get.docker.com/ | sh

Follow the instructions in the terminal. Please note that the installer may ask
you to perform additional steps, such as copying the client to the `/usr/bin`
folder.

If you would like to use docker with your normal user run the following as root,
replacing `your-user` with your login user name:

    # usermod -aG docker your-user

You may need to logout and back in for the above changes to be made.

Once installed restart your machine or start the daemon manually with:

    # service docker start

To verify that everything has worked as expected:

    $ docker run --rm hello-world

This command downloads and runs the `hello-world` image in a container. When the
container runs, it prints an informational message. Then, it exits.

> **Note**:
> If you want to enable memory and swap accounting see
> [this](/installation/ubuntulinux/#memory-and-swap-accounting).

### Uninstallation

To uninstall the Docker package:

    $ sudo apt-get purge lxc-docker

To uninstall the Docker package and dependencies that are no longer needed:

    $ sudo apt-get autoremove --purge lxc-docker

The above commands will not remove images, containers, volumes, or user created
configuration files on your host. If you wish to delete all images, containers,
and volumes run the following command:

    $ rm -rf /var/lib/docker

You must delete the user created configuration files manually.

## Giving non-root access

The `docker` daemon always runs as the `root` user and the `docker`
daemon binds to a Unix socket instead of a TCP port. By default that
Unix socket is owned by the user `root`, and so, by default, you can
access it with `sudo`.

If you (or your Docker installer) create a Unix group called `docker`
and add users to it, then the `docker` daemon will make the ownership of
the Unix socket read/writable by the `docker` group when the daemon
starts. The `docker` daemon must always run as the root user, but if you
run the `docker` client as a user in the `docker` group then you don't
need to add `sudo` to all the client commands. From Docker 0.9.0 you can
use the `-G` flag to specify an alternative group.

> **Warning**:
> The `docker` group (or the group specified with the `-G` flag) is
> `root`-equivalent; see [*Docker Daemon Attack Surface*](
> /articles/security/#docker-daemon-attack-surface) details.

**Example:**

    # Add the docker group if it doesn't already exist.
    $ sudo groupadd docker

    # Add the connected user "${USER}" to the docker group.
    # Change the user name to match your preferred user.
    # You may have to logout and log back in again for
    # this to take effect.
    $ sudo gpasswd -a ${USER} docker

    # Restart the Docker daemon.
    $ sudo service docker restart


## What next?

Continue with the [User Guide](/userguide/).

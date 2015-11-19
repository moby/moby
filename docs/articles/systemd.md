<!--[metadata]>
+++
title = "Control and configure Docker with systemd"
description = "Controlling and configuring Docker using systemd"
keywords = ["docker, daemon, systemd,  configuration"]
[menu.main]
parent = "smn_administrate"
weight = 7
+++
<![end-metadata]-->

# Control and configure Docker with systemd

Many Linux distributions use systemd to start the Docker daemon. This document
shows a few examples of how to customise Docker's settings.

## Starting the Docker daemon

Once Docker is installed, you will need to start the Docker daemon.

    $ sudo systemctl start docker
    # or on older distributions, you may need to use
    $ sudo service docker start

If you want Docker to start at boot, you should also:

    $ sudo systemctl enable docker
    # or on older distributions, you may need to use
    $ sudo chkconfig docker on

## Custom Docker daemon options

There are a number of ways to configure the daemon flags and environment variables
for your Docker daemon.

The recommended way is to use a systemd drop-in file. These are local files in
the `/etc/systemd/system/docker.service.d` directory. This could also be
`/etc/systemd/system/docker.service`, which also works for overriding the
defaults from `/lib/systemd/system/docker.service`.

However, if you had previously used a package which had an `EnvironmentFile`
(often pointing to `/etc/sysconfig/docker`) then for backwards compatibility,
you drop a file in the `/etc/systemd/system/docker.service.d`
directory including the following:

    [Service]
    EnvironmentFile=-/etc/sysconfig/docker
    EnvironmentFile=-/etc/sysconfig/docker-storage
    EnvironmentFile=-/etc/sysconfig/docker-network
    ExecStart=
    ExecStart=/usr/bin/docker daemon -H fd:// $OPTIONS \
              $DOCKER_STORAGE_OPTIONS \
              $DOCKER_NETWORK_OPTIONS \
              $BLOCK_REGISTRY \
              $INSECURE_REGISTRY

To check if the `docker.service` uses an `EnvironmentFile`:

    $ sudo systemctl show docker | grep EnvironmentFile
    EnvironmentFile=-/etc/sysconfig/docker (ignore_errors=yes)

Alternatively, find out where the service file is located:

    $ sudo systemctl status docker | grep Loaded
       Loaded: loaded (/usr/lib/systemd/system/docker.service; enabled)
    $ sudo grep EnvironmentFile /usr/lib/systemd/system/docker.service
    EnvironmentFile=-/etc/sysconfig/docker

You can customize the Docker daemon options using override files as explained in the
[HTTP Proxy example](#http-proxy) below. The files located in `/usr/lib/systemd/system`
or `/lib/systemd/system` contain the default options and should not be edited.

### Runtime directory and storage driver

You may want to control the disk space used for Docker images, containers
and volumes by moving it to a separate partition.

In this example, we'll assume that your `docker.service` file looks something like:

    [Unit]
    Description=Docker Application Container Engine
    Documentation=https://docs.docker.com
    After=network.target docker.socket
    Requires=docker.socket

    [Service]
    Type=notify
    ExecStart=/usr/bin/docker daemon -H fd://
    LimitNOFILE=1048576
    LimitNPROC=1048576

    [Install]
    Also=docker.socket

This will allow us to add extra flags via a drop-in file (mentioned above) by
placing a file containing the following in the `/etc/systemd/system/docker.service.d`
directory:

    [Service]
    ExecStart=
    ExecStart=/usr/bin/docker daemon -H fd:// --graph /mnt/docker-data --storage-driver btrfs

You can also set other environment variables in this file, for example, the
`HTTP_PROXY` environment variables described below.

To modify the ExecStart configuration, specify an empty configuration followed
by a new configuration as follows:

    [Service]
    ExecStart=
    ExecStart=/usr/bin/docker daemon -H fd:// --bip=172.17.42.1/16

If you fail to specify an empty configuration, Docker reports an error such as:

    docker.service has more than one ExecStart= setting, which is only allowed for Type=oneshot services. Refusing.

### HTTP proxy

This example overrides the default `docker.service` file.

If you are behind a HTTP proxy server, for example in corporate settings,
you will need to add this configuration in the Docker systemd service file.

First, create a systemd drop-in directory for the docker service:

    mkdir /etc/systemd/system/docker.service.d

Now create a file called `/etc/systemd/system/docker.service.d/http-proxy.conf`
that adds the `HTTP_PROXY` environment variable:

    [Service]
    Environment="HTTP_PROXY=http://proxy.example.com:80/"

If you have internal Docker registries that you need to contact without
proxying you can specify them via the `NO_PROXY` environment variable:

    Environment="HTTP_PROXY=http://proxy.example.com:80/" "NO_PROXY=localhost,127.0.0.1,docker-registry.somecorporation.com"

Flush changes:

    $ sudo systemctl daemon-reload

Verify that the configuration has been loaded:

    $ sudo systemctl show docker --property Environment
    Environment=HTTP_PROXY=http://proxy.example.com:80/

Restart Docker:

    $ sudo systemctl restart docker

## Manually creating the systemd unit files

When installing the binary without a package, you may want
to integrate Docker with systemd. For this, simply install the two unit files
(service and socket) from [the github
repository](https://github.com/docker/docker/tree/master/contrib/init/systemd)
to `/etc/systemd/system`.

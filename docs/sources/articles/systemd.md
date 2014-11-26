page_title: Controlling and configuring Docker using Systemd
page_description: Controlling and configuring Docker using Systemd
page_keywords: docker, daemon, systemd, configuration

# Controlling and configuring Docker using Systemd

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

If the `docker.service` file is set to use an `EnvironmentFile`
(often pointing to `/etc/sysconfig/docker`) then you can modify the
referenced file.

Or, you may need to edit the `docker.service` file, which can be in `/usr/lib/systemd/system`
or `/etc/systemd/service`.

### Runtime directory and storage driver

You may want to control the disk space used for Docker images, containers
and volumes by moving it to a separate partition.

In this example, we'll assume that your `docker.services` file looks something like:

    [Unit]
    Description=Docker Application Container Engine
    Documentation=http://docs.docker.com
    After=network.target docker.socket
    Requires=docker.socket
    
    [Service]
    Type=notify
    EnvironmentFile=-/etc/sysconfig/docker
    ExecStart=/usr/bin/docker -d -H fd:// $OPTIONS
    LimitNOFILE=1048576
    LimitNPROC=1048576
    
    [Install]
    Also=docker.socket

This will allow us to add extra flags to the `/etc/sysconfig/docker` file by
setting `OPTIONS`:

    OPTIONS="--graph /mnt/docker-data --storage btrfs"

You can also set other environment variables in this file, for example, the
`HTTP_PROXY` environment variables described below.

### HTTP Proxy

This example overrides the default `docker.service` file.

If you are behind a HTTP proxy server, for example in corporate settings, 
you will need to add this configuration in the Docker systemd service file.

Copy file `/usr/lib/systemd/system/docker.service` to `/etc/systemd/system/docker/service`.

Add the following to the `[Service]` section in the new file:

    Environment="HTTP_PROXY=http://proxy.example.com:80/"

If you have internal Docker registries that you need to contact without
proxying you can specify them via the `NO_PROXY` environment variable:

    Environment="HTTP_PROXY=http://proxy.example.com:80/" "NO_PROXY=localhost,127.0.0.0/8,docker-registry.somecorporation.com"

Flush changes:

    $ sudo systemctl daemon-reload
    
Restart Docker:

    $ sudo systemctl restart docker

## Manually creating the systemd unit files

When installing the binary without a package, you may want
to integrate Docker with systemd. For this, simply install the two unit files
(service and socket) from [the github
repository](https://github.com/docker/docker/tree/master/contrib/init/systemd)
to `/etc/systemd/system`.



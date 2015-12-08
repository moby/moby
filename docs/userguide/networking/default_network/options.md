<!--[metadata]>
+++
draft=true
title = "Tools and Examples"
keywords = ["docker, bridge, docker0, network"]
[menu.main]
parent = "smn_networking_def"
+++
<![end-metadata]-->

<!--[metadata]>
We may want to add it back in later under another form. Labeled DRAFT for now. Won't be built.
<![end-metadata]-->

# Quick guide to the options
Here is a quick list of the networking-related Docker command-line options, in case it helps you find the section below that you are looking for.

Some networking command-line options can only be supplied to the Docker server when it starts up, and cannot be changed once it is running:
- `-b BRIDGE` or `--bridge=BRIDGE` -- see

  [Building your own bridge](#bridge-building)

- `--bip=CIDR` -- see

  [Customizing docker0](#docker0)

- `--default-gateway=IP_ADDRESS` -- see

  [How Docker networks a container](#container-networking)

- `--default-gateway-v6=IP_ADDRESS` -- see

  [IPv6](#ipv6)

- `--fixed-cidr` -- see

  [Customizing docker0](#docker0)

- `--fixed-cidr-v6` -- see

  [IPv6](#ipv6)

- `-H SOCKET...` or `--host=SOCKET...` --

  This might sound like it would affect container networking,

  but it actually faces in the other direction:

  it tells the Docker server over what channels

  it should be willing to receive commands

  like "run container" and "stop container."

- `--icc=true|false` -- see

  [Communication between containers](#between-containers)

- `--ip=IP_ADDRESS` -- see

  [Binding container ports](#binding-ports)

- `--ipv6=true|false` -- see

  [IPv6](#ipv6)

- `--ip-forward=true|false` -- see

  [Communication between containers and the wider world](#the-world)

- `--iptables=true|false` -- see

  [Communication between containers](#between-containers)

- `--mtu=BYTES` -- see

  [Customizing docker0](#docker0)

- `--userland-proxy=true|false` -- see

  [Binding container ports](#binding-ports)

There are three networking options that can be supplied either at startup or when `docker run` is invoked.  When provided at startup, set the default value that `docker run` will later use if the options are not specified:
- `--dns=IP_ADDRESS...` -- see

  [Configuring DNS](#dns)

- `--dns-search=DOMAIN...` -- see

  [Configuring DNS](#dns)

- `--dns-opt=OPTION...` -- see

  [Configuring DNS](#dns)

Finally, several networking options can only be provided when calling `docker run` because they specify something specific to one container:
- `-h HOSTNAME` or `--hostname=HOSTNAME` -- see

  [Configuring DNS](#dns) and

  [How Docker networks a container](#container-networking)

- `--link=CONTAINER_NAME_or_ID:ALIAS` -- see

  [Configuring DNS](#dns) and

  [Communication between containers](#between-containers)

- `--net=bridge|none|container:NAME_or_ID|host` -- see

  [How Docker networks a container](#container-networking)

- `--mac-address=MACADDRESS...` -- see

  [How Docker networks a container](#container-networking)

- `-p SPEC` or `--publish=SPEC` -- see

  [Binding container ports](#binding-ports)

- `-P` or `--publish-all=true|false` -- see

  [Binding container ports](#binding-ports)

To supply networking options to the Docker server at startup, use the `DOCKER_OPTS` variable in the Docker upstart configuration file. For Ubuntu, edit the variable in `/etc/default/docker` or `/etc/sysconfig/docker` for CentOS.

The following example illustrates how to configure Docker on Ubuntu to recognize a newly built bridge.

Edit the `/etc/default/docker` file:

```
$ echo 'DOCKER_OPTS="-b=bridge0"' >> /etc/default/docker
```

Then restart the Docker server.

```
$ sudo service docker start
```

For additional information on bridges, see [building your own bridge](#building-your-own-bridge) later on this page.

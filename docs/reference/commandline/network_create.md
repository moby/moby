<!--[metadata]>
+++
title = "network create"
description = "The network create command description and usage"
keywords = ["network, create"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# network create

    Usage:  docker network create [OPTIONS] NETWORK-NAME

    Creates a new network with a name specified by the user

    --aux-address=map[]      Auxiliary ipv4 or ipv6 addresses used by network driver
    -d --driver=DRIVER       Driver to manage the Network bridge or overlay. The default is bridge.
    --gateway=[]             ipv4 or ipv6 Gateway for the master subnet
    --help                   Print usage
    --internal               Restricts external access to the network
    --ip-range=[]            Allocate container ip from a sub-range
    --ipam-driver=default    IP Address Management Driver
    --ipam-opt=map[]         Set custom IPAM driver specific options
    --ipv6                   Enable IPv6 networking
    --label=[]               Set metadata on a network
    -o --opt=map[]           Set custom driver specific options
    --subnet=[]              Subnet in CIDR format that represents a network segment

Creates a new network. The `DRIVER` accepts `bridge` or `overlay` which are the
built-in network drivers. If you have installed a third party or your own custom
network driver you can specify that `DRIVER` here also. If you don't specify the
`--driver` option, the command automatically creates a `bridge` network for you.
When you install Docker Engine it creates a `bridge` network automatically. This
network corresponds to the `docker0` bridge that Engine has traditionally relied
on. When launch a new container with  `docker run` it automatically connects to
this bridge network. You cannot remove this default bridge network but you can
create new ones using the `network create` command.

```bash
$ docker network create -d bridge my-bridge-network
```

Bridge networks are isolated networks on a single Engine installation. If you
want to create a network that spans multiple Docker hosts each running an
Engine, you must create an `overlay` network. Unlike `bridge` networks overlay
networks require some pre-existing conditions before you can create one. These
conditions are:

* Access to a key-value store. Engine supports Consul, Etcd, and ZooKeeper (Distributed store) key-value stores.
* A cluster of hosts with connectivity to the key-value store.
* A properly configured Engine `daemon` on each host in the cluster.

The `docker daemon` options that support the `overlay` network are:

* `--cluster-store`
* `--cluster-store-opt`
* `--cluster-advertise`

To read more about these options and how to configure them, see ["*Get started
with multi-host network*"](../../userguide/networking/get-started-overlay.md).

It is also a good idea, though not required, that you install Docker Swarm on to
manage the cluster that makes up your network. Swarm provides sophisticated
discovery and server management that can assist your implementation.

Once you have prepared the `overlay` network prerequisites you simply choose a
Docker host in the cluster and issue the following to create the network:

```bash
$ docker network create -d overlay my-multihost-network
```

Network names must be unique. The Docker daemon attempts to identify naming
conflicts but this is not guaranteed. It is the user's responsibility to avoid
name conflicts.

## Connect containers

When you start a container use the `--net` flag to connect it to a network.
This adds the `busybox` container to the `mynet` network.

```bash
$ docker run -itd --net=mynet busybox
```

If you want to add a container to a network after the container is already
running use the `docker network connect` subcommand.

You can connect multiple containers to the same network. Once connected, the
containers can communicate using only another container's IP address or name.
For `overlay` networks or custom plugins that support multi-host connectivity,
containers connected to the same multi-host network but launched from different
Engines can also communicate in this way.

You can disconnect a container from a network using the `docker network
disconnect` command.

## Specifying advanced options

When you create a network, Engine creates a non-overlapping subnetwork for the network by default. This subnetwork is not a subdivision of an existing network. It is purely for ip-addressing purposes. You can override this default and specify subnetwork values directly using the `--subnet` option. On a `bridge` network you can only create a single subnet:

```bash
docker network create --driver=bridge --subnet=192.168.0.0/16 br0
```
Additionally, you also specify the `--gateway` `--ip-range` and `--aux-address` options.

```bash
network create --driver=bridge --subnet=172.28.0.0/16 --ip-range=172.28.5.0/24 --gateway=172.28.5.254 br0
```

If you omit the `--gateway` flag the Engine selects one for you from inside a
preferred pool. For `overlay` networks and for network driver plugins that
support it you can create multiple subnetworks.

```bash
docker network create -d overlay
  --subnet=192.168.0.0/16 --subnet=192.170.0.0/16
  --gateway=192.168.0.100 --gateway=192.170.0.100
  --ip-range=192.168.1.0/24
  --aux-address a=192.168.1.5 --aux-address b=192.168.1.6
  --aux-address a=192.170.1.5 --aux-address b=192.170.1.6
  my-multihost-network
```
Be sure that your subnetworks do not overlap. If they do, the network create fails and Engine returns an error.

# Bridge driver options

When creating a custom network, the default network driver (i.e. `bridge`) has additional options that can be passed.
The following are those options and the equivalent docker daemon flags used for docker0 bridge:

| Option                                           | Equivalent  | Description                                           |
|--------------------------------------------------|-------------|-------------------------------------------------------|
| `com.docker.network.bridge.name`                 | -           | bridge name to be used when creating the Linux bridge |
| `com.docker.network.bridge.enable_ip_masquerade` | `--ip-masq` | Enable IP masquerading                                |
| `com.docker.network.bridge.enable_icc`           | `--icc`     | Enable or Disable Inter Container Connectivity        |
| `com.docker.network.bridge.host_binding_ipv4`    | `--ip`      | Default IP when binding container ports               |
| `com.docker.network.mtu`                         | `--mtu`     | Set the containers network MTU                        |

The following arguments can be passed to `docker network create` for any network driver, again with their approximate
equivalents to `docker daemon`.

| Argument     | Equivalent     | Description                                |
|--------------|----------------|--------------------------------------------|
| `--gateway`  | -              | ipv4 or ipv6 Gateway for the master subnet |
| `--ip-range` | `--fixed-cidr` | Allocate IPs from a range                  |
| `--internal` | -              | Restricts external access to the network   |
| `--ipv6`     | `--ipv6`       | Enable IPv6 networking                     |
| `--subnet`   | `--bip`        | Subnet for network                         |

For example, let's use `-o` or `--opt` options to specify an IP address binding when publishing ports:

```bash
docker network create -o "com.docker.network.bridge.host_binding_ipv4"="172.19.0.1" simple-network
```

### Network internal mode

By default, when you connect a container to an `overlay` network, Docker also connects a bridge network to it to provide external connectivity.
If you want to create an externally isolated `overlay` network, you can specify the `--internal` option.

## Related information

* [network inspect](network_inspect.md)
* [network connect](network_connect.md)
* [network disconnect](network_disconnect.md)
* [network ls](network_ls.md)
* [network rm](network_rm.md)
* [Understand Docker container networks](../../userguide/networking/dockernetworks.md)

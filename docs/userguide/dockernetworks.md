<!--[metadata]>
+++
title = "Docker container networking"
description = "How do we connect docker containers within and across hosts ?"
keywords = ["Examples, Usage, network, docker, documentation, user guide, multihost, cluster"]
[menu.main]
parent = "smn_containers"
weight = 3
+++
<![end-metadata]-->

# Docker container networking

So far we've been introduced to some [basic Docker
concepts](usingdocker.md), seen how to work with [Docker
images](dockerimages.md) as well as learned about basic [networking
and links between containers](dockerlinks.md). In this section
we're going to discuss how you can take control over more advanced 
container networking.

This section makes use of `docker network` commands and outputs to explain the
advanced networking functionality supported by Docker.

# Default Networks

By default, docker creates 3 networks using 3 different network drivers :

```
$ sudo docker network ls
NETWORK ID          NAME                DRIVER
7fca4eb8c647        bridge              bridge
9f904ee27bf5        none                null
cf03ee007fb4        host                host
```

`docker network inspect` gives more information about a network

```
$ sudo docker network inspect bridge
{
    "name": "bridge",
    "id": "7fca4eb8c647e57e9d46c32714271e0c3f8bf8d17d346629e2820547b2d90039",
    "driver": "bridge",
    "containers": {}
}
```

By default containers are launched on Bridge network

```
$ sudo docker run -itd --name=container1 busybox
f2870c98fd504370fb86e59f32cd0753b1ac9b69b7d80566ffc7192a82b3ed27

$ sudo docker run -itd --name=container2 busybox
bda12f8922785d1f160be70736f26c1e331ab8aaf8ed8d56728508f2e2fd4727
```

```
$ sudo docker network inspect bridge
{
    "name": "bridge",
    "id": "7fca4eb8c647e57e9d46c32714271e0c3f8bf8d17d346629e2820547b2d90039",
    "driver": "bridge",
    "containers": {
        "bda12f8922785d1f160be70736f26c1e331ab8aaf8ed8d56728508f2e2fd4727": {
            "endpoint": "e0ac95934f803d7e36384a2029b8d1eeb56cb88727aa2e8b7edfeebaa6dfd758",
            "mac_address": "02:42:ac:11:00:03",
            "ipv4_address": "172.17.0.3/16",
            "ipv6_address": ""
        },
        "f2870c98fd504370fb86e59f32cd0753b1ac9b69b7d80566ffc7192a82b3ed27": {
            "endpoint": "31de280881d2a774345bbfb1594159ade4ae4024ebfb1320cb74a30225f6a8ae",
            "mac_address": "02:42:ac:11:00:02",
            "ipv4_address": "172.17.0.2/16",
            "ipv6_address": ""
        }
    }
}
```
`docker network inspect` command above shows all the connected containers and its network resources on a given network

Containers in a network should be able to communicate with each other using container names

```
$ sudo docker attach container1

/ # ifconfig
eth0      Link encap:Ethernet  HWaddr 02:42:AC:11:00:02
          inet addr:172.17.0.2  Bcast:0.0.0.0  Mask:255.255.0.0
          inet6 addr: fe80::42:acff:fe11:2/64 Scope:Link
          UP BROADCAST RUNNING MULTICAST  MTU:1500  Metric:1
          RX packets:17 errors:0 dropped:0 overruns:0 frame:0
          TX packets:3 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:0
          RX bytes:1382 (1.3 KiB)  TX bytes:258 (258.0 B)

lo        Link encap:Local Loopback
          inet addr:127.0.0.1  Mask:255.0.0.0
          inet6 addr: ::1/128 Scope:Host
          UP LOOPBACK RUNNING  MTU:65536  Metric:1
          RX packets:0 errors:0 dropped:0 overruns:0 frame:0
          TX packets:0 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:0
          RX bytes:0 (0.0 B)  TX bytes:0 (0.0 B)

/ # ping container2
PING container2 (172.17.0.3): 56 data bytes
64 bytes from 172.17.0.3: seq=0 ttl=64 time=0.125 ms
64 bytes from 172.17.0.3: seq=1 ttl=64 time=0.130 ms
64 bytes from 172.17.0.3: seq=2 ttl=64 time=0.172 ms
^C
--- container2 ping statistics ---
3 packets transmitted, 3 packets received, 0% packet loss
round-trip min/avg/max = 0.125/0.142/0.172 ms

/ # cat /etc/hosts
172.17.0.2      f2870c98fd50
127.0.0.1       localhost
::1     localhost ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
172.17.0.2      container1
172.17.0.2      container1.bridge
172.17.0.3      container2
172.17.0.3      container2.bridge
```


```
$ sudo docker attach container2

/ # ifconfig
eth0      Link encap:Ethernet  HWaddr 02:42:AC:11:00:03
          inet addr:172.17.0.3  Bcast:0.0.0.0  Mask:255.255.0.0
          inet6 addr: fe80::42:acff:fe11:3/64 Scope:Link
          UP BROADCAST RUNNING MULTICAST  MTU:1500  Metric:1
          RX packets:8 errors:0 dropped:0 overruns:0 frame:0
          TX packets:8 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:0
          RX bytes:648 (648.0 B)  TX bytes:648 (648.0 B)

lo        Link encap:Local Loopback
          inet addr:127.0.0.1  Mask:255.0.0.0
          inet6 addr: ::1/128 Scope:Host
          UP LOOPBACK RUNNING  MTU:65536  Metric:1
          RX packets:0 errors:0 dropped:0 overruns:0 frame:0
          TX packets:0 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:0
          RX bytes:0 (0.0 B)  TX bytes:0 (0.0 B)

/ # ping container1
PING container1 (172.17.0.2): 56 data bytes
64 bytes from 172.17.0.2: seq=0 ttl=64 time=0.277 ms
64 bytes from 172.17.0.2: seq=1 ttl=64 time=0.179 ms
64 bytes from 172.17.0.2: seq=2 ttl=64 time=0.130 ms
64 bytes from 172.17.0.2: seq=3 ttl=64 time=0.113 ms
^C
--- container1 ping statistics ---
4 packets transmitted, 4 packets received, 0% packet loss
round-trip min/avg/max = 0.113/0.174/0.277 ms
/ # cat /etc/hosts
172.17.0.3      bda12f892278
127.0.0.1       localhost
::1     localhost ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
172.17.0.2      container1
172.17.0.2      container1.bridge
172.17.0.3      container2
172.17.0.3      container2.bridge
/ #

```

# User defined Networks

In addition to the inbuilt networks, user can create  networks using inbuilt drivers
(such as bridge or overlay driver) or external plugins supplied by the community.
Networks by definition should provides complete isolation for the containers.

```
$ docker network create -d bridge isolated_nw
8b05faa32aeb43215f67678084a9c51afbdffe64cd91e3f5bb8267475f8bf1a7

$ docker network inspect isolated_nw
{
    "name": "isolated_nw",
    "id": "8b05faa32aeb43215f67678084a9c51afbdffe64cd91e3f5bb8267475f8bf1a7",
    "driver": "bridge",
    "containers": {}
}

$ docker network ls
NETWORK ID          NAME                DRIVER
9f904ee27bf5        none                null
cf03ee007fb4        host                host
7fca4eb8c647        bridge              bridge
8b05faa32aeb        isolated_nw         bridge

```

Container can be launched on a user-defined network using the --net=<NETWORK> option 
in `docker run` command

```
$ docker run --net=isolated_nw -itd --name=container3 busybox
777344ef4943d34827a3504a802bf15db69327d7abe4af28a05084ca7406f843

$ docker network inspect isolated_nw
{
    "name": "isolated_nw",
    "id": "8b05faa32aeb43215f67678084a9c51afbdffe64cd91e3f5bb8267475f8bf1a7",
    "driver": "bridge",
    "containers": {
        "777344ef4943d34827a3504a802bf15db69327d7abe4af28a05084ca7406f843": {
            "endpoint": "c7f22f8da07fb8ecc687d08377cfcdb80b4dd8624c2a8208b1a4268985e38683",
            "mac_address": "02:42:ac:14:00:01",
            "ipv4_address": "172.20.0.1/16",
            "ipv6_address": ""
        }
    }
}
```


# Connecting to Multiple networks

Docker containers can dynamically connect to 1 or more networks with each network backed
by same or different network driver / plugin.

```
$ docker network connect isolated_nw container2
$ docker network inspect isolated_nw
{
    "name": "isolated_nw",
    "id": "8b05faa32aeb43215f67678084a9c51afbdffe64cd91e3f5bb8267475f8bf1a7",
    "driver": "bridge",
    "containers": {
        "777344ef4943d34827a3504a802bf15db69327d7abe4af28a05084ca7406f843": {
            "endpoint": "c7f22f8da07fb8ecc687d08377cfcdb80b4dd8624c2a8208b1a4268985e38683",
            "mac_address": "02:42:ac:14:00:01",
            "ipv4_address": "172.20.0.1/16",
            "ipv6_address": ""
        },
        "bda12f8922785d1f160be70736f26c1e331ab8aaf8ed8d56728508f2e2fd4727": {
            "endpoint": "2ac11345af68b0750341beeda47cc4cce93bb818d8eb25e61638df7a4997cb1b",
            "mac_address": "02:42:ac:14:00:02",
            "ipv4_address": "172.20.0.2/16",
            "ipv6_address": ""
        }
    }
}
```

Lets check the network resources used by container2.

```
$ docker inspect --format='{{.NetworkSettings.Networks}}' container2
[bridge isolated_nw]

$ sudo docker attach container2

/ # ifconfig
eth0      Link encap:Ethernet  HWaddr 02:42:AC:11:00:03
          inet addr:172.17.0.3  Bcast:0.0.0.0  Mask:255.255.0.0
          inet6 addr: fe80::42:acff:fe11:3/64 Scope:Link
          UP BROADCAST RUNNING MULTICAST  MTU:1500  Metric:1
          RX packets:21 errors:0 dropped:0 overruns:0 frame:0
          TX packets:18 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:0
          RX bytes:1586 (1.5 KiB)  TX bytes:1460 (1.4 KiB)

eth1      Link encap:Ethernet  HWaddr 02:42:AC:14:00:02
          inet addr:172.20.0.2  Bcast:0.0.0.0  Mask:255.255.0.0
          inet6 addr: fe80::42:acff:fe14:2/64 Scope:Link
          UP BROADCAST RUNNING MULTICAST  MTU:1500  Metric:1
          RX packets:8 errors:0 dropped:0 overruns:0 frame:0
          TX packets:8 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:0
          RX bytes:648 (648.0 B)  TX bytes:648 (648.0 B)

lo        Link encap:Local Loopback
          inet addr:127.0.0.1  Mask:255.0.0.0
          inet6 addr: ::1/128 Scope:Host
          UP LOOPBACK RUNNING  MTU:65536  Metric:1
          RX packets:0 errors:0 dropped:0 overruns:0 frame:0
          TX packets:0 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:0
          RX bytes:0 (0.0 B)  TX bytes:0 (0.0 B)
```


In the example discussed in this section  thus far, container3 and container2 are 
connected to isolated_nw and can talk to each other. 
But container3 and container1 are not in the same network and hence they cannot communicate.

```
$ docker attach container3

/ # ifconfig
eth0      Link encap:Ethernet  HWaddr 02:42:AC:14:00:01
          inet addr:172.20.0.1  Bcast:0.0.0.0  Mask:255.255.0.0
          inet6 addr: fe80::42:acff:fe14:1/64 Scope:Link
          UP BROADCAST RUNNING MULTICAST  MTU:1500  Metric:1
          RX packets:24 errors:0 dropped:0 overruns:0 frame:0
          TX packets:8 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:0
          RX bytes:1944 (1.8 KiB)  TX bytes:648 (648.0 B)

lo        Link encap:Local Loopback
          inet addr:127.0.0.1  Mask:255.0.0.0
          inet6 addr: ::1/128 Scope:Host
          UP LOOPBACK RUNNING  MTU:65536  Metric:1
          RX packets:0 errors:0 dropped:0 overruns:0 frame:0
          TX packets:0 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:0
          RX bytes:0 (0.0 B)  TX bytes:0 (0.0 B)

/ # ping container2.isolated_nw
PING container2.isolated_nw (172.20.0.2): 56 data bytes
64 bytes from 172.20.0.2: seq=0 ttl=64 time=0.217 ms
64 bytes from 172.20.0.2: seq=1 ttl=64 time=0.150 ms
64 bytes from 172.20.0.2: seq=2 ttl=64 time=0.188 ms
64 bytes from 172.20.0.2: seq=3 ttl=64 time=0.176 ms
^C
--- container2.isolated_nw ping statistics ---
4 packets transmitted, 4 packets received, 0% packet loss
round-trip min/avg/max = 0.150/0.182/0.217 ms
/ # ping container2
PING container2 (172.20.0.2): 56 data bytes
64 bytes from 172.20.0.2: seq=0 ttl=64 time=0.120 ms
64 bytes from 172.20.0.2: seq=1 ttl=64 time=0.109 ms
^C
--- container2 ping statistics ---
2 packets transmitted, 2 packets received, 0% packet loss
round-trip min/avg/max = 0.109/0.114/0.120 ms

/ # ping container1
ping: bad address 'container1'

/ # ping 172.17.0.2
PING 172.17.0.2 (172.17.0.2): 56 data bytes
^C
--- 172.17.0.2 ping statistics ---
4 packets transmitted, 0 packets received, 100% packet loss

/ # ping 172.17.0.3
PING 172.17.0.3 (172.17.0.3): 56 data bytes
^C
--- 172.17.0.3 ping statistics ---
4 packets transmitted, 0 packets received, 100% packet loss

```

While container2 is attached to both the networks (bridge and isolated_nw) and hence it 
can talk to both container1 and container3

```
$ docker attach container2

/ # cat /etc/hosts
172.17.0.3      bda12f892278
127.0.0.1       localhost
::1     localhost ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
172.17.0.2      container1
172.17.0.2      container1.bridge
172.17.0.3      container2
172.17.0.3      container2.bridge
172.20.0.1      container3
172.20.0.1      container3.isolated_nw
172.20.0.2      container2
172.20.0.2      container2.isolated_nw

/ # ping container3
PING container3 (172.20.0.1): 56 data bytes
64 bytes from 172.20.0.1: seq=0 ttl=64 time=0.138 ms
64 bytes from 172.20.0.1: seq=1 ttl=64 time=0.133 ms
64 bytes from 172.20.0.1: seq=2 ttl=64 time=0.133 ms
^C
--- container3 ping statistics ---
3 packets transmitted, 3 packets received, 0% packet loss
round-trip min/avg/max = 0.133/0.134/0.138 ms

/ # ping container1
PING container1 (172.17.0.2): 56 data bytes
64 bytes from 172.17.0.2: seq=0 ttl=64 time=0.121 ms
64 bytes from 172.17.0.2: seq=1 ttl=64 time=0.250 ms
64 bytes from 172.17.0.2: seq=2 ttl=64 time=0.133 ms
^C
--- container1 ping statistics ---
3 packets transmitted, 3 packets received, 0% packet loss
round-trip min/avg/max = 0.121/0.168/0.250 ms
/ #
```


Just like it is easy to connect a container to multiple networks,  one can 
disconnect a container from a network using the `docker network disconnect` command.

```
root@Ubuntu-vm ~$ docker network disconnect isolated_nw container2

$ docker inspect --format='{{.NetworkSettings.Networks}}' container2
[bridge]

root@Ubuntu-vm ~$ docker network inspect isolated_nw
{
    "name": "isolated_nw",
    "id": "8b05faa32aeb43215f67678084a9c51afbdffe64cd91e3f5bb8267475f8bf1a7",
    "driver": "bridge",
    "containers": {
        "777344ef4943d34827a3504a802bf15db69327d7abe4af28a05084ca7406f843": {
            "endpoint": "c7f22f8da07fb8ecc687d08377cfcdb80b4dd8624c2a8208b1a4268985e38683",
            "mac_address": "02:42:ac:14:00:01",
            "ipv4_address": "172.20.0.1/16",
            "ipv6_address": ""
        }
    }
}
```

Once a container is disconnected from a network, it cannot communicate with other containers
connected to that network. In this example, container2 cannot talk to container3 any more 
in isolated_nw

```
$ sudo docker attach container2

/ # ifconfig
eth0      Link encap:Ethernet  HWaddr 02:42:AC:11:00:03
          inet addr:172.17.0.3  Bcast:0.0.0.0  Mask:255.255.0.0
          inet6 addr: fe80::42:acff:fe11:3/64 Scope:Link
          UP BROADCAST RUNNING MULTICAST  MTU:1500  Metric:1
          RX packets:26 errors:0 dropped:0 overruns:0 frame:0
          TX packets:23 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:0
          RX bytes:1964 (1.9 KiB)  TX bytes:1838 (1.7 KiB)

lo        Link encap:Local Loopback
          inet addr:127.0.0.1  Mask:255.0.0.0
          inet6 addr: ::1/128 Scope:Host
          UP LOOPBACK RUNNING  MTU:65536  Metric:1
          RX packets:0 errors:0 dropped:0 overruns:0 frame:0
          TX packets:0 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:0
          RX bytes:0 (0.0 B)  TX bytes:0 (0.0 B)

/ # ping container3
PING container3 (172.20.0.1): 56 data bytes
^C
--- container3 ping statistics ---
2 packets transmitted, 0 packets received, 100% packet loss


But container2 still has full connectivity to the bridge network

/ # ping container1
PING container1 (172.17.0.2): 56 data bytes
64 bytes from 172.17.0.2: seq=0 ttl=64 time=0.119 ms
64 bytes from 172.17.0.2: seq=1 ttl=64 time=0.174 ms
^C
--- container1 ping statistics ---
2 packets transmitted, 2 packets received, 0% packet loss
round-trip min/avg/max = 0.119/0.146/0.174 ms
/ #

```

When all the containers in a network stops or disconnected the network can be removed

```
$ docker network inspect isolated_nw
{
    "name": "isolated_nw",
    "id": "8b05faa32aeb43215f67678084a9c51afbdffe64cd91e3f5bb8267475f8bf1a7",
    "driver": "bridge",
    "containers": {}
}

$ docker network rm isolated_nw

$ docker network ls
NETWORK ID          NAME                DRIVER
9f904ee27bf5        none                null
cf03ee007fb4        host                host
7fca4eb8c647        bridge              bridge
```

# Native Multi-host networking

With the help of libnetwork and the inbuilt `VXLAN based overlay network driver` docker supports multi-host networking natively out of the box. Technical details are documented under https://github.com/docker/libnetwork/blob/master/docs/overlay.md.
Using the exact same above `docker network` UI, the user can exercise the power of multi-host networking.

In order to create a network using the inbuilt overlay driver,

```
$ docker network create -d overlay multi-host-network
```

Since `network` object is globally significant, this feature requires distributed states provided by `libkv`. Using `libkv`, the user can plug any of the supported Key-Value store (such as consul, etcd or zookeeper).
User can specify the Key-Value store of choice using the `--cluster-store` daemon flag, which takes configuration value of format `PROVIDER://URL`, where
`PROVIDER` is the name of the Key-Value store (such as consul, etcd or zookeeper) and
`URL` is the url to reach the Key-Value store.
Example : `docker daemon --cluster-store=consul://localhost:8500`

# Next step

Now that you know how to link Docker containers together, the next step is
learning how to manage data, volumes and mounts inside your containers.

Go to [Managing Data in Containers](dockervolumes.md).

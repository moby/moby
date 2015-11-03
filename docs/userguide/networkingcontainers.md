<!--[metadata]>
+++
title = "Networking containers"
description = "How to manage data inside your Docker containers."
keywords = ["Examples, Usage, volume, docker, documentation, user guide, data,  volumes"]
[menu.main]
parent = "smn_containers"
weight = -3
+++
<![end-metadata]-->


# Networking containers

If you are working your way through the user guide, you just built and ran a
simple application. You've also built in your own images. This section teaches
you how to network your containers.

## Name a container

You've already seen that each container you create has an automatically
created name; indeed you've become familiar with our old friend
`nostalgic_morse` during this guide. You can also name containers
yourself. This naming provides two useful functions:

*  You can name containers that do specific functions in a way
   that makes it easier for you to remember them, for example naming a
   container containing a web application `web`.

*  Names provide Docker with a reference point that allows it to refer to other
   containers. There are several commands that support this and you'll use one in a exercise later.

You name your container by using the `--name` flag, for example launch a new container called web:

    $ docker run -d -P --name web training/webapp python app.py

Use the `docker ps` command to see check the name:

    $ docker ps -l
    CONTAINER ID  IMAGE                  COMMAND        CREATED       STATUS       PORTS                    NAMES
    aed84ee21bde  training/webapp:latest python app.py  12 hours ago  Up 2 seconds 0.0.0.0:49154->5000/tcp  web

You can also use `docker inspect` with the container's name.

    $ docker inspect web
    [
    {
        "Id": "3ce51710b34f5d6da95e0a340d32aa2e6cf64857fb8cdb2a6c38f7c56f448143",
        "Created": "2015-10-25T22:44:17.854367116Z",
        "Path": "python",
        "Args": [
            "app.py"
        ],
        "State": {
            "Status": "running",
            "Running": true,
            "Paused": false,
            "Restarting": false,
            "OOMKilled": false,
      ...

Container names must be unique. That means you can only call one container
`web`. If you want to re-use a container name you must delete the old container
(with `docker rm`) before you can reuse the name with a new container. Go ahead and stop and them remove your `web` container.

    $ docker stop web
    web
    $ docker rm web
    web


## Launch a container on the default network

Docker includes support for networking containers through the use of **network
drivers**. By default, Docker provides two network drivers for you, the
`bridge` and the `overlay` driver. You can also write a network driver plugin so
that you can create your own drivers but that is an advanced task.

Every installation of the Docker Engine automatically includes three default networks. You can list them:

    $ docker network ls
    NETWORK ID          NAME                DRIVER
    18a2866682b8        none                null                
    c288470c46f6        host                host                
    7b369448dccb        bridge              bridge  

The network named `bridge` is a special network. Unless you tell it otherwise, Docker always launches your containers in this network. Try this now:

    $ docker run -itd --name=networktest ubuntu
    74695c9cea6d9810718fddadc01a727a5dd3ce6a69d09752239736c030599741

Inspecting the network is an easy way to find out the container's IP address.

```bash
[
    {
        "Name": "bridge",
        "Id": "f7ab26d71dbd6f557852c7156ae0574bbf62c42f539b50c8ebde0f728a253b6f",
        "Scope": "local",
        "Driver": "bridge",
        "IPAM": {
            "Driver": "default",
            "Config": [
                {
                    "Subnet": "172.17.0.1/16",
                    "Gateway": "172.17.0.1"
                }
            ]
        },
        "Containers": {
            "3386a527aa08b37ea9232cbcace2d2458d49f44bb05a6b775fba7ddd40d8f92c": {
                "EndpointID": "647c12443e91faf0fd508b6edfe59c30b642abb60dfab890b4bdccee38750bc1",
                "MacAddress": "02:42:ac:11:00:02",
                "IPv4Address": "172.17.0.2/16",
                "IPv6Address": ""
            },
            "94447ca479852d29aeddca75c28f7104df3c3196d7b6d83061879e339946805c": {
                "EndpointID": "b047d090f446ac49747d3c37d63e4307be745876db7f0ceef7b311cbba615f48",
                "MacAddress": "02:42:ac:11:00:03",
                "IPv4Address": "172.17.0.3/16",
                "IPv6Address": ""
            }
        },
        "Options": {
            "com.docker.network.bridge.default_bridge": "true",
            "com.docker.network.bridge.enable_icc": "true",
            "com.docker.network.bridge.enable_ip_masquerade": "true",
            "com.docker.network.bridge.host_binding_ipv4": "0.0.0.0",
            "com.docker.network.bridge.name": "docker0",
            "com.docker.network.driver.mtu": "9001"
        }
    }
]
```

You can remove a container from a network by disconnecting the container. To do this, you supply both the network name and the container name. You can also use the container id. In this example, though, the name is faster.

    $ docker network disconnect bridge networktest

While you can disconnect a container from a network, you cannot remove the  builtin `bridge` network named `bridge`. Networks are natural ways to isolate containers from other containers or other networks. So, as you get more experienced with Docker, you'll want to create your own networks.

## Create your own bridge network

Docker Engine natively supports both bridge networks and overlay networks. A bridge network is limited to a single host running Docker Engine. An overlay network can include multiple hosts and is a more advanced topic. For this example, you'll create a bridge network:  

    $ docker network create -d bridge my-bridge-network

The `-d` flag tells Docker to use the `bridge` driver for the new network. You could have left this flag off as `bridge` is the default value for this flag. Go ahead and list the networks on your machine:

    $ docker network ls
    NETWORK ID          NAME                DRIVER
    7b369448dccb        bridge              bridge              
    615d565d498c        my-bridge-network   bridge              
    18a2866682b8        none                null                
    c288470c46f6        host                host

If you inspect the network, you'll find that it has nothing in it.

    $ docker network inspect my-bridge-network
    [
        {
            "Name": "my-bridge-network",
            "Id": "5a8afc6364bccb199540e133e63adb76a557906dd9ff82b94183fc48c40857ac",
            "Scope": "local",
            "Driver": "bridge",
            "IPAM": {
                "Driver": "default",
                "Config": [
                    {}
                ]
            },
            "Containers": {},
            "Options": {}
        }
    ]

## Add containers to a network

To build web applications that act in concert but do so securely, create a
network. Networks, by definition, provide complete isolation for containers. You
can add containers to a network when you first run a container.

Launch a container running a PostgreSQL database and pass it the `--net=my-bridge-network` flag to connect it to your new network:

    $ docker run -d --net=my-bridge-network --name db training/postgres

If you inspect your `my-bridge-network` you'll see it has a container attached.
You can also inspect your container to see where it is connected:

    $ docker inspect --format='{{json .NetworkSettings.Networks}}'  db
    {"bridge":{"EndpointID":"508b170d56b2ac9e4ef86694b0a76a22dd3df1983404f7321da5649645bf7043","Gateway":"172.17.0.1","IPAddress":"172.17.0.3","IPPrefixLen":16,"IPv6Gateway":"","GlobalIPv6Address":"","GlobalIPv6PrefixLen":0,"MacAddress":"02:42:ac:11:00:02"}}

Now, go ahead and start your by now familiar web application. This time leave off the `-P` flag and also don't specify a network.

    $ docker run -d --name web training/webapp python app.py

Which network is your `web` application running under? Inspect the application and you'll find it is running in the default `bridge` network.

    $ docker inspect --format='{{json .NetworkSettings.Networks}}'  web
    {"bridge":{"EndpointID":"508b170d56b2ac9e4ef86694b0a76a22dd3df1983404f7321da5649645bf7043","Gateway":"172.17.0.1","IPAddress":"172.17.0.3","IPPrefixLen":16,"IPv6Gateway":"","GlobalIPv6Address":"","GlobalIPv6PrefixLen":0,"MacAddress":"02:42:ac:11:00:02"}}

Then, get the IP address of your `web`

    $ docker inspect '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' web
    172.17.0.2

Now, open a shell to your running `db` container:

    $ docker exec -it db bash
    root@a205f0dd33b2:/# ping 172.17.0.2
    ping 172.17.0.2
    PING 172.17.0.2 (172.17.0.2) 56(84) bytes of data.
    ^C
    --- 172.17.0.2 ping statistics ---
    44 packets transmitted, 0 received, 100% packet loss, time 43185ms

After a bit, use CTRL-C to end the `ping` and you'll find the ping failed. That is because the two container are running on different networks. You can fix that. Then, use CTRL-C to exit the container.

Docker networking allows you to attach a container to as many networks as you like. You can also attach an already running container. Go ahead and attach your running `web` app to the `my-bridge-network`.

    $ docker network connect my-bridge-network Web

Open a shell into the `db` application again and try the ping command. This time just use the container name `web` rather than the IP Address.

    $ docker exec -it db bash
    root@a205f0dd33b2:/# ping web
    PING web (172.19.0.3) 56(84) bytes of data.
    64 bytes from web (172.19.0.3): icmp_seq=1 ttl=64 time=0.095 ms
    64 bytes from web (172.19.0.3): icmp_seq=2 ttl=64 time=0.060 ms
    64 bytes from web (172.19.0.3): icmp_seq=3 ttl=64 time=0.066 ms
    ^C
    --- web ping statistics ---
    3 packets transmitted, 3 received, 0% packet loss, time 2000ms
    rtt min/avg/max/mdev = 0.060/0.073/0.095/0.018 ms

The `ping` shows it is contacting a different IP address, the address on the `my-bridge-network` which is different from its address on the `bridge` network.

## Next steps

Now that you know how to network containers, see [how to manage data in containers](dockervolumes.md).

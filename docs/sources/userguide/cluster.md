page_title: Getting started with Docker Cluster
page_description: Introductory guide to getting a Docker cluster setup
page_keywords: documentation, docs, cluster, docker multi host, scheduling

# Getting Started with Docker Cluster


This section provides a quick introduction to getting a Docker cluster setup
on your infrastructure.

## Discovery

Before we can start deploying our container's to a Docker cluster we need to 
ensure that each of the nodes are able to discover each other.  The Hub
allows you to manage your Docker cluster easily.  To start a new cluster 
login to your Hub account and hit **Create Cluster** providing a name
of your choosing.  Under my account I will create a new cluster named
**cluster-1**.  After creating a cluster you should be able to see and manage 
the nodes that are currently registerd.  After creating a cluster you 
should receive a URL that looks similar to 
`https://discovery.hub.docker.com/u/demo-user/cluster-1` with your Hub 
username and cluster name.


To add a new or existing Docker
Engine to your newly created cluster use the provided URL from the Hub
with the `--discovery` flag when you start Docker in daemon mode.

    $ docker -d --discovery https://discovery.hub.docker.com/u/demo-user/cluster-1


In the future, other discovery mechanism will be provided that will not depend on the Hub.

## Master

In order to ensure consistency within the cluster one of your Docker Engines
will need to be promoted to master within the cluster.  If you are using
the Hub's discovery service you will be able to promote any of your registered
nodes to a master from the web interface.  You can also statically assign one
of the Docker Engines with the `--master` flag.

    $ docker -d --master --discovery https://discovery.hub.docker.com/u/demo-user/cluster-1

Eventually, clustering will provide a built-in leader election algorithm making the `--master` flag obsolete.


## Nodes

Alongside the master, we will add additional nodes in the cluster that will be able to accept tasks issued by the master.
Those nodes simply have to be started with the `--discovery` flag in order to join the cluster.
For this guide we will start a three node cluster with each node's hostname being, **node-1**, **node-2**, and **node-3**.  


## Running your first containers

To deploy a container to the cluster you can use your existing Docker CLI to issue a
run command against the master node.

    $ export DOCKER_HOST=tcp://master-address:1234
    $ docker run -d -P --name redis  redis
    redis

To view the Docker Engine that is running our container we can run `docker ps` to view
that information.  

    $ docker ps
    CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS                           NODE        NAMES
    f8b693db9cd6        redis:2.8           "redis-server"      Up About a minute   running             192.168.0.42:49178->6379/tcp    node-1      redis

The commands you would use for single host will work in clustering mode.

```
$ docker port redis
6379/tcp -> 192.168.0.42:49178
```
```
$ docker inspect redis
...
"Ports": {
        "6379/tcp": [
            {
                "HostIp": "192.168.0.42",
                "HostPort": "49178"
            }
        ]
    }
...
```

```    
$ docker logs redis
[1] 29 Oct 00:45:43.323 # Warning: no config file specified, using the default config. In order to specify a config file use redis-server /path/to/redis.conf
                _._
           _.-``__ ''-._
      _.-``    `.  `_.  ''-._           Redis 2.8.17 (00000000/0) 64 bit
  .-`` .-```.  ```\/    _.,_ ''-._
 (    '      ,       .-`  | `,    )     Running in stand alone mode
 |`-._`-...-` __...-.``-._|'` _.-'|     Port: 6379
 |    `-._   `._    /     _.-'    |     PID: 1
  `-._    `-._  `-./  _.-'    _.-'
 |`-._`-._    `-.__.-'    _.-'_.-'|
 |    `-._`-._        _.-'_.-'    |           http://redis.io
  `-._    `-._`-.__.-'_.-'    _.-'
 |`-._`-._    `-.__.-'    _.-'_.-'|
 |    `-._`-._        _.-'_.-'    |
  `-._    `-._`-.__.-'_.-'    _.-'
      `-._    `-.__.-'    _.-'
          `-._        _.-'
              `-.__.-'

[1] 29 Oct 00:45:43.325 # Server started, Redis version 2.8.17
[1] 29 Oct 00:45:43.325 # WARNING overcommit_memory is set to 0! Background save may fail under low memory condition. To fix this issue add 'vm.overcommit_memory = 1' to /etc/sysctl.conf and then reboot or run the command 'sysctl vm.overcommit_memory=1' for this to take effect.
[1] 29 Oct 00:45:43.325 * The server is now ready to accept connections on port 6379
```

## Resources

In order to schedule your container to a node within the cluster that has enough capacity
to run it, you need to provide resource requirements such as *CPU* and/or *RAM*.


These requirements are used to place your container on a machine with enough resources
available run it.  If there are currently no nodes within the cluster
that satisfy your requirements, the request will be rejected.

Let's say we have 3 nodes, each with **1CPU** and **2GB** of ram.

```
$ docker run -d -P -m 10g redis
2014/10/29 00:33:20 Error response from daemon: no resources availalble to schedule container
```

We don't have room to schedule a container with 10GB of ram, let's try **1GB**:
```
$ docker run -d -P -m 1g redis
f8b693db9cd6

$ docker ps
CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS                           NODE        NAMES
f8b693db9cd6        redis:2.8           "redis-server"      Up About a minute   running             192.168.0.42:49178->6379/tcp    node-1      prickly_engelbart
```

The default scheduler uses bin packing to avoid resource fragmentation. If we ask for **1GB** of ram again, the container will be placed on the same node:
```
$ docker run -d -P -m 1g redis
963841b138d8

$ docker ps
CONTAINER ID        IMAGE               COMMAND             CREATED                  STATUS              PORTS                           NODE        NAMES
963841b138d8        redis:2.8           "redis-server"      Less than a second ago   running             192.168.0.42:49177->6379/tcp    node-1      dreamy_turing
f8b693db9cd6        redis:2.8           "redis-server"      Up About a minute        running             192.168.0.42:49178->6379/tcp    node-1      prickly_engelbart
```

Once the node is full, clustering will move onto the next available:
```
$ docker run -d -P -m 1g redis
87c4376856a8

$ docker ps
CONTAINER ID        IMAGE               COMMAND             CREATED                  STATUS              PORTS                           NODE        NAMES
87c4376856a8        redis:2.8           "redis-server"      Less than a second ago   running             192.168.0.43:49177->6379/tcp    node-2      stoic_albattani
963841b138d8        redis:2.8           "redis-server"      Up About a minute        running             192.168.0.42:49177->6379/tcp    node-1      dreamy_turing
f8b693db9cd6        redis:2.8           "redis-server"      Up About a minute        running             192.168.0.42:49178->6379/tcp    node-1      prickly_engelbart
```

Beside *CPU* and *RAM*, everything that is unique is considered as a resource by the scheduler.

For the time being, this includes *ports*:
```
$ docker run -d -p 80:80 nginx
87c4376856a8

$ docker ps
CONTAINER ID        IMAGE               COMMAND             CREATED                  STATUS              PORTS                           NODE        NAMES
87c4376856a8        nginx:latest        "nginx"             Less than a second ago   running             192.168.0.42:80->80/tcp         node-1      prickly_engelbart
```

Docker cluster selects a node where the public `80` port is available and schedules a container on it, in this case `node-1`.

Attempting to run another container with the public `80` port will result in clustering selecting a different node, since that port is already occupied on `node-1`:
```
$ docker run -d -p 80:80 nginx
963841b138d8

$ docker ps
CONTAINER ID        IMAGE               COMMAND             CREATED                  STATUS              PORTS                           NODE        NAMES
963841b138d8        nginx:latest        "nginx"             Less than a second ago   running             192.168.0.43:80->80/tcp         node-2      dreamy_turing
87c4376856a8        nginx:latest        "nginx"             Up About a minute        running             192.168.0.42:80->80/tcp         node-1      prickly_engelbart
```

Again, repeating the same command will result in the selection of `node-3`, since port `80` is neither available on `node-1` nor `node-2`:
```
$ docker run -d -p 80:80 nginx
963841b138d8

$ docker ps
CONTAINER ID        IMAGE               COMMAND             CREATED                  STATUS              PORTS                           NODE        NAMES
f8b693db9cd6        nginx:latest        "nginx"             Less than a second ago   running             192.168.0.44:80->80/tcp         node-3      stoic_albattani
963841b138d8        nginx:latest        "nginx"             Up About a minute        running             192.168.0.43:80->80/tcp         node-2      dreamy_turing
87c4376856a8        nginx:latest        "nginx"             Up About a minute        running             192.168.0.42:80->80/tcp         node-1      prickly_engelbart
```

Finally, Docker Cluster will refuse to run another container that requires port `80` since not a single node in the cluster has it available:
```
$ docker run -d -p 80:80 nginx
2014/10/29 00:33:20 Error response from daemon: no resources availalble to schedule container
```

## Constraints

Constraints are key/value pairs associated to particular nodes. You can see them as *node tags*.

When creating a container, the user can select a subset of nodes that should be considered
for scheduling by specifying one or more sets of matching key/value pairs.

This approach has several practical use cases such as:
* Selecting specific host properties (such as `storage=ssd`, in order to schedule containers on specific hardware).
* Tagging nodes based on their physical location (`region=us-east`, to force containers to run on a given location).
* Logical cluster partioning (`environment=production`, to split a cluster into sub-clusters with different properties).

To tag a node with a specific set of key/value pairs, one must pass a list of `--constraint` options at node startup time.

For instance, let's start `node-1` with the `storage=ssd` tag:
```
$ docker -d --discovery https://discovery.hub.docker.com/u/demo-user/cluster-1 --constraint storage=ssd
```

Again, but this time `node-2` with `storage=disk`:
```
$ docker -d --discovery https://discovery.hub.docker.com/u/demo-user/cluster-1 --constraint storage=disk
```

Once the nodes are registered with the cluster, the master pulls their respective tags and will take them into account when scheduling new containers.

Let's start a MySQL server and make sure it gets good I/O performance by selecting nodes with flash drives:

```
$ docker run -d -P --constraint storage=ssd --name db mysql
f8b693db9cd6

$ docker ps
CONTAINER ID        IMAGE               COMMAND             CREATED                  STATUS              PORTS                           NODE        NAMES
f8b693db9cd6        mysql:latest        "mysqld"            Less than a second ago   running             192.168.0.42:49178->3306/tcp    node-1      db
```

In this case, the master selected all nodes that met the `storage=ssd` constraint and applied resource management on top of them, as discussed earlier.
`node-1` was selected in this example since it's the only host running flash.

Now we want to run an `nginx` frontend in our cluster. However, we don't want *flash* drives since we'll mostly write logs to disk.

```
$ docker run -d -P --constraint storage=disk --name frontend nginx
f8b693db9cd6

$ docker ps
CONTAINER ID        IMAGE               COMMAND             CREATED                  STATUS              PORTS                           NODE        NAMES
963841b138d8        nginx:latest        "nginx"             Less than a second ago   running             192.168.0.43:49177->80/tcp      node-2      frontend
f8b693db9cd6        mysql:latest        "mysqld"            Up About a minute        running             192.168.0.42:49178->3306/tcp    node-1      db
```

The scheduler selected `node-2` since it was started with the `storage=disk` tag.

### Standard Constraints

Additionally, a standard set of constraints can be used when scheduling containers without specifying them when starting the node.
Those tags are sourced from `docker info` and currently include:

* OperatingSystem
* KernelVersion
* Driver
* ExecutionDriver


## Rebalancing

One of the benefits of a distributed system is that it handles failover gracefully.
If one of the nodes within your Docker cluster fails for some reason the master is
able to reschedule your containers to another machine. Lets shutdown our **node-1** 
to see how Docker handles an entire node failure.  

    $ ssh node-1 && poweroff

After querying docker we should see our container go through a few states then finally
be redeployed to a new machine.

    $ docker ps -a
    CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS                           NODE        NAMES
    f8b693db9cd6        redis:2.8           "redis-server"      About a minute      exited              192.168.0.42:49178->6379/tcp    node-1      stoic_albattani 

    $ docker ps -a
    CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS                           NODE        NAMES
    f8b693db9cd6        redis:2.8           "redis-server"      About a minute      pending                                                         

    $ docker ps -a
    CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS                           NODE        NAMES
    f8b693db9cd6        redis:2.8           "redis-server"      About a minute      running             192.168.0.43:49178->6379/tcp    node-2      stoic_albattani

Upon rebalancing, the scheduler will look at the shape of your container (resource requirements, contraints...) and search for a node available.
If there is no such node, the container will remain in `pending` state until all conditions are met.

Not all containers need or may be rescheduled. For those cases, the rescheduling behaviour can
be controlled through a special `constraint`, named `reschedule`:

    $ docker run -d -P --constraint storage=ssd --constraint reschedule=never mysql
    $ docker run -d -P --constraint reschedule=always nginx

This is particular useful for **stateful** containers, that is, a container with state attached to a particular machine.

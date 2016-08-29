<!--[metadata]>
+++
title = "Deploy services to a swarm"
description = "Deploy services to a swarm"
keywords = ["guide", "swarm mode", "swarm", "service"]
[menu.main]
identifier="services-guide"
parent="engine_swarm"
weight=15
+++
<![end-metadata]-->

# Deploy services to a swarm

When you are running Docker Engine in swarm mode, you run
`docker service create` to deploy your application in the swarm. The swarm
manager accepts the service description as the desired state for your
application. The built-in swarm orchestrator and scheduler deploy your
application to nodes in your swarm to achieve and maintain the desired state.

For an overview of how services work, refer to [How services work](how-swarm-mode-works/services.md).

This guide assumes you are working with the Docker Engine running in swarm
mode. You must run all `docker service` commands from a manager node.

If you haven't already, read through [Swarm mode key concepts](key-concepts.md)
and [How services work](how-swarm-mode-works/services.md).

## Create a service

To create the simplest type of service in a swarm, you only need to supply
a container image:

```bash
$ docker service create <IMAGE>
```

The swarm orchestrator schedules one task on an available node. The task invokes
a container based upon the image. For example, you could run the following
command to create a service of one instance of an nginx web server:

```bash
$ docker service create --name my_web nginx

anixjtol6wdfn6yylbkrbj2nx
```

In this example the `--name` flag names the service `my_web`.

To list the service, run `docker service ls` from a manager node:

```bash
$ docker service ls

ID            NAME    REPLICAS  IMAGE  COMMAND
anixjtol6wdf  my_web  1/1       nginx
```

To make the web server accessible from outside the swarm, you need to
[publish the port](#publish-ports-externally-to-the-swarm) where the swarm
listens for web requests.

You can include a command to run inside containers after the image:

```bash
$ docker service create <IMAGE> <COMMAND>
```

For example to start an `alpine` image that runs `ping docker.com`:

```bash
$ docker service create --name helloworld alpine ping docker.com

9uk4639qpg7npwf3fn2aasksr
```

## Configure the runtime environment

You can configure the following options for the runtime environment in the
container:

* environment variables using the `--env` flag
* the working directory inside the container using the `--workdir` flag
* the username or UID using the `--user` flag

For example:

```bash
$ docker service create --name helloworld \
  --env MYVAR=myvalue \
  --workdir /tmp \
  --user my_user \
  alpine ping docker.com

9uk4639qpg7npwf3fn2aasksr
```

## Control service scale and placement

Swarm mode has two types of services, replicated and global. For replicated
services, you specify the number of replica tasks for the swarm manager to
schedule onto available nodes. For global services, the scheduler places one
task on each available node.

You control the type of service using the `--mode` flag. If you don't specify a
mode, the service defaults to `replicated`. For replicated services, you specify
the number of replica tasks you want to start using the `--replicas` flag. For
example, to start a replicated nginx service with 3 replica tasks:

```bash
$ docker service create --name my_web --replicas 3 nginx
```

To start a global service on each available node, pass `--mode global` to
`docker service create`. Every time a new node becomes available, the scheduler
places a task for the global service on the new node. For example to start a
service that runs alpine on every node in the swarm:

```bash
$ docker service create --name myservice --mode global alpine top
```

Service constraints let you set criteria for a node to meet before the scheduler
deploys a service to the node. You can apply constraints to the
service based upon node attributes and metadata or engine metadata. For more
information on constraints, refer to the `docker service create` [CLI  reference](../reference/commandline/service_create.md).


## Configure service networking options

Swarm mode lets you network services in a couple of ways:

* publish ports externally to the swarm using ingress networking
* connect services and tasks within the swarm using overlay networks

### Publish ports externally to the swarm

You publish service ports externally to the swarm using the `--publish
<TARGET-PORT>:<SERVICE-PORT>` flag. When you publish a service port, the swarm
makes the service accessible at the target port on every node regardless if
there is a task for the service running on the node.

For example, imagine you want to deploy a 3-replica nginx service to a 10-node
swarm as follows:

```bash
docker service create --name my_web --replicas 3 --publish 8080:80 nginx
```

The scheduler will deploy nginx tasks to a maximum of 3 nodes. However, the
swarm makes nginx port 80 from the task container accessible at port 8080 on any
node in the swarm. You can direct `curl` at port 8080 of any node in the swarm
to access the web server:

```bash
$ curl localhost:8080

<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
    body {
        width: 35em;
        margin: 0 auto;
        font-family: Tahoma, Verdana, Arial, sans-serif;
    }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>

<p>For online documentation and support please refer to
<a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at
<a href="http://nginx.com/">nginx.com</a>.</p>

<p><em>Thank you for using nginx.</em></p>
</body>
</html>
```

### Add an overlay network

Use overlay networks to connect one or more services within the swarm.

First, create an overlay network on a manager node the `docker network create`
command:

```bash
$ docker network create --driver overlay my-network

etjpu59cykrptrgw0z0hk5snf
```

After you create an overlay network in swarm mode, all manager nodes have access
to the network.

When you create a service and pass the `--network` flag to attach the service to
the overlay network:

```bash
$ docker service create \
  --replicas 3 \
  --network my-network \
  --name my-web \
  nginx

716thylsndqma81j6kkkb5aus
```

The swarm extends `my-network` to each node running the service.

For more information on overlay networking and service discovery, refer to
[Attach services to an overlay network](networking.md). See also
[Docker swarm mode overlay network security model](../userguide/networking/overlay-security-model.md).

## Configure update behavior

When you create a service, you can specify a rolling update behavior for how the
swarm should apply changes to the service when you run `docker service update`.
You can also specify these flags as part of the update, as arguments to
`docker service update`.

The `--update-delay` flag configures the time delay between updates to a service
task or sets of tasks. You can describe the time `T` as a combination of the
number of seconds `Ts`, minutes `Tm`, or hours `Th`. So `10m30s` indicates a 10
minute 30 second delay.

By default the scheduler updates 1 task at a time. You can pass the
`--update-parallelism` flag to configure the maximum number of service tasks
that the scheduler updates simultaneously.

When an update to an individual task returns a state of `RUNNING`, the scheduler
continues the update by continuing to another task until all tasks are updated.
If, at any time during an update a task returns `FAILED`, the scheduler pauses
the update. You can control the behavior using the `--update-failure-action`
flag for `docker service create` or `docker service update`.

In the example service below, the scheduler applies updates to a maximum of 2
replicas at a time. When an updated task returns either `RUNNING` or `FAILED`,
the scheduler waits 10 seconds before stopping the next task to update:

```bash
$ docker service create \
  --replicas 10 \
  --name my_web \
  --update-delay 10s \
  --update-parallelism 2 \
  --update-failure-action continue \
  alpine

0u6a4s31ybk7yw2wyvtikmu50
```

## Configure mounts

You can create two types of mounts for services in a swarm, `volume` mounts or
`bind` mounts. You pass the `--mount` flag when you create a service. The
default is a volume mount if you don't specify a type.

* Volumes are storage that remain alive after a container for a task has
been removed. The preferred method to mount volumes is to leverage an existing
volume:

```bash
$ docker service create \
  --mount src=<VOLUME-NAME>,dst=<CONTAINER-PATH> \
  --name myservice \
  <IMAGE>
```

For more information on how to create a volume, see the `volume create` [CLI reference](../reference/commandline/volume_create.md).

The following method creates the volume at deployment time when the scheduler
dispatches a task, just before the starting the container:

```bash
$ docker service create \
  --mount type=volume,src=<VOLUME-NAME>,dst=<CONTAINER-PATH>,volume-driver=<DRIVER>,volume-opt=<KEY0>=<VALUE0>,volume-opt=<KEY1>=<VALUE1>
  --name myservice \
  <IMAGE>
```

* Bind mounts are file system paths from the host where the scheduler deploys
the container for the task. Docker mounts the path into the container. The
file system path must exist before the swarm initializes the container for the
task.

The following examples show bind mount syntax:

```bash
# Mount a read-write bind
$ docker service create \
  --mount type=bind,src=<HOST-PATH>,dst=<CONTAINER-PATH> \
  --name myservice \
  <IMAGE>

# Mount a read-only bind
$ docker service create \
  --mount type=bind,src=<HOST-PATH>,dst=<CONTAINER-PATH>,readonly \
  --name myservice \
  <IMAGE>
```

>**Important note:** Bind mounts can be useful but they are also dangerous.  In
most cases, we recommend that you architect your application such that mounting
paths from the host is unnecessary. The main risks include the following:<br />
> <br />
> If you bind mount a host path into your service’s containers, the path
> must exist on every machine. The Docker swarm mode scheduler can schedule
> containers on any machine that meets resource availability requirements
> and satisfies all `--constraint`s you specify.<br />
> <br />
> The Docker swarm mode scheduler may reschedule your running service
> containers at any time if they become unhealthy or unreachable.<br />
> <br />
> Host bind mounts are completely non-portable.  When you use  bind mounts,
> there is no guarantee that your application will run the same way in
> development as it does in production.


## Learn More

* [Swarm administration guide](admin_guide.md)
* [Docker Engine command line reference](../reference/commandline/index.md)
* [Swarm mode tutorial](swarm-tutorial/index.md)

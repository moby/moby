---
title: "service create"
description: "The service create command description and usage"
keywords: "service, create"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# service create

```Markdown
Usage:  docker service create [OPTIONS] IMAGE [COMMAND] [ARG...]

Create a new service

Options:
      --constraint list                    Placement constraints
      --container-label list               Container labels
  -d, --detach                             Exit immediately instead of waiting for the service to converge (default true)
      --dns list                           Set custom DNS servers
      --dns-option list                    Set DNS options
      --dns-search list                    Set custom DNS search domains
      --endpoint-mode string               Endpoint mode (vip or dnsrr) (default "vip")
      --entrypoint command                 Overwrite the default ENTRYPOINT of the image
  -e, --env list                           Set environment variables
      --env-file list                      Read in a file of environment variables
      --group list                         Set one or more supplementary user groups for the container
      --health-cmd string                  Command to run to check health
      --health-interval duration           Time between running the check (ns|us|ms|s|m|h)
      --health-retries int                 Consecutive failures needed to report unhealthy
      --health-start-period duration       Start period for the container to initialize before counting retries towards unstable (ns|us|ms|s|m|h)
      --health-timeout duration            Maximum time to allow one check to run (ns|us|ms|s|m|h)
      --help                               Print usage
      --host list                          Set one or more custom host-to-IP mappings (host:ip)
      --hostname string                    Container hostname
  -l, --label list                         Service labels
      --limit-cpu decimal                  Limit CPUs
      --limit-memory bytes                 Limit Memory
      --log-driver string                  Logging driver for service
      --log-opt list                       Logging driver options
      --mode string                        Service mode (replicated or global) (default "replicated")
      --mount mount                        Attach a filesystem mount to the service
      --name string                        Service name
      --network list                       Network attachments
      --no-healthcheck                     Disable any container-specified HEALTHCHECK
      --placement-pref pref                Add a placement preference
  -p, --publish port                       Publish a port as a node port
  -q, --quiet                              Suppress progress output
      --read-only                          Mount the container's root filesystem as read only
      --replicas uint                      Number of tasks
      --reserve-cpu decimal                Reserve CPUs
      --reserve-memory bytes               Reserve Memory
      --restart-condition string           Restart when condition is met ("none"|"on-failure"|"any") (default "any")
      --restart-delay duration             Delay between restart attempts (ns|us|ms|s|m|h) (default 5s)
      --restart-max-attempts uint          Maximum number of restarts before giving up
      --restart-window duration            Window used to evaluate the restart policy (ns|us|ms|s|m|h)
      --rollback-delay duration            Delay between task rollbacks (ns|us|ms|s|m|h) (default 0s)
      --rollback-failure-action string     Action on rollback failure ("pause"|"continue") (default "pause")
      --rollback-max-failure-ratio float   Failure rate to tolerate during a rollback (default 0)
      --rollback-monitor duration          Duration after each task rollback to monitor for failure (ns|us|ms|s|m|h) (default 5s)
      --rollback-order string              Rollback order ("start-first"|"stop-first") (default "stop-first")
      --rollback-parallelism uint          Maximum number of tasks rolled back simultaneously (0 to roll back all at once) (default 1)
      --secret secret                      Specify secrets to expose to the service
      --stop-grace-period duration         Time to wait before force killing a container (ns|us|ms|s|m|h) (default 10s)
      --stop-signal string                 Signal to stop the container
  -t, --tty                                Allocate a pseudo-TTY
      --update-delay duration              Delay between updates (ns|us|ms|s|m|h) (default 0s)
      --update-failure-action string       Action on update failure ("pause"|"continue"|"rollback") (default "pause")
      --update-max-failure-ratio float     Failure rate to tolerate during an update (default 0)
      --update-monitor duration            Duration after each task update to monitor for failure (ns|us|ms|s|m|h) (default 5s)
      --update-order string                Update order ("start-first"|"stop-first") (default "stop-first")
      --update-parallelism uint            Maximum number of tasks updated simultaneously (0 to update all at once) (default 1)
  -u, --user string                        Username or UID (format: <name|uid>[:<group|gid>])
      --with-registry-auth                 Send registry authentication details to swarm agents
  -w, --workdir string                     Working directory inside the container
```

## Description

Creates a service as described by the specified parameters. You must run this
command on a manager node.

## Examples

### Create a service

```bash
$ docker service create --name redis redis:3.0.6

dmu1ept4cxcfe8k8lhtux3ro3

$ docker service create --mode global --name redis2 redis:3.0.6

a8q9dasaafudfs8q8w32udass

$ docker service ls

ID            NAME    MODE        REPLICAS  IMAGE
dmu1ept4cxcf  redis   replicated  1/1       redis:3.0.6
a8q9dasaafud  redis2  global      1/1       redis:3.0.6
```

### Create a service with 5 replica tasks (--replicas)

Use the `--replicas` flag to set the number of replica tasks for a replicated
service. The following command creates a `redis` service with `5` replica tasks:

```bash
$ docker service create --name redis --replicas=5 redis:3.0.6

4cdgfyky7ozwh3htjfw0d12qv
```

The above command sets the *desired* number of tasks for the service. Even
though the command returns immediately, actual scaling of the service may take
some time. The `REPLICAS` column shows both the *actual* and *desired* number
of replica tasks for the service.

In the following example the desired state is  `5` replicas, but the current
number of `RUNNING` tasks is `3`:

```bash
$ docker service ls

ID            NAME   MODE        REPLICAS  IMAGE
4cdgfyky7ozw  redis  replicated  3/5       redis:3.0.7
```

Once all the tasks are created and `RUNNING`, the actual number of tasks is
equal to the desired number:

```bash
$ docker service ls

ID            NAME   MODE        REPLICAS  IMAGE
4cdgfyky7ozw  redis  replicated  5/5       redis:3.0.7
```

### Create a service with secrets

Use the `--secret` flag to give a container access to a
[secret](secret_create.md).

Create a service specifying a secret:

```bash
$ docker service create --name redis --secret secret.json redis:3.0.6

4cdgfyky7ozwh3htjfw0d12qv
```

Create a service specifying the secret, target, user/group ID and mode:

```bash
$ docker service create --name redis \
    --secret source=ssh-key,target=ssh \
    --secret source=app-key,target=app,uid=1000,gid=1001,mode=0400 \
    redis:3.0.6

4cdgfyky7ozwh3htjfw0d12qv
```

Secrets are located in `/run/secrets` in the container.  If no target is
specified, the name of the secret will be used as the in memory file in the
container.  If a target is specified, that will be the filename.  In the
example above, two files will be created: `/run/secrets/ssh` and
`/run/secrets/app` for each of the secret targets specified.

### Create a service with a rolling update policy

```bash
$ docker service create \
  --replicas 10 \
  --name redis \
  --update-delay 10s \
  --update-parallelism 2 \
  redis:3.0.6
```

When you run a [service update](service_update.md), the scheduler updates a
maximum of 2 tasks at a time, with `10s` between updates. For more information,
refer to the [rolling updates
tutorial](https://docs.docker.com/engine/swarm/swarm-tutorial/rolling-update/).

### Set environment variables (-e, --env)

This sets environmental variables for all tasks in a service. For example:

```bash
$ docker service create --name redis_2 --replicas 5 --env MYVAR=foo redis:3.0.6
```

### Create a service with specific hostname (--hostname)

This option sets the docker service containers hostname to a specific string.
For example:

```bash
$ docker service create --name redis --hostname myredis redis:3.0.6
```

### Set metadata on a service (-l, --label)

A label is a `key=value` pair that applies metadata to a service. To label a
service with two labels:

```bash
$ docker service create \
  --name redis_2 \
  --label com.example.foo="bar"
  --label bar=baz \
  redis:3.0.6
```

For more information about labels, refer to [apply custom
metadata](https://docs.docker.com/engine/userguide/labels-custom-metadata/).

### Add bind-mounts or volumes

Docker supports two different kinds of mounts, which allow containers to read to
or write from files or directories on other containers or the host operating
system. These types are _data volumes_ (often referred to simply as volumes) and
_bind-mounts_.

Additionally, Docker supports `tmpfs` mounts.

A **bind-mount** makes a file or directory on the host available to the
container it is mounted within. A bind-mount may be either read-only or
read-write. For example, a container might share its host's DNS information by
means of a bind-mount of the host's `/etc/resolv.conf` or a container might
write logs to its host's `/var/log/myContainerLogs` directory. If you use
bind-mounts and your host and containers have different notions of permissions,
access controls, or other such details, you will run into portability issues.

A **named volume** is a mechanism for decoupling persistent data needed by your
container from the image used to create the container and from the host machine.
Named volumes are created and managed by Docker, and a named volume persists
even when no container is currently using it. Data in named volumes can be
shared between a container and the host machine, as well as between multiple
containers. Docker uses a _volume driver_ to create, manage, and mount volumes.
You can back up or restore volumes using Docker commands.

A **tmpfs** mounts a tmpfs inside a container for volatile data.

Consider a situation where your image starts a lightweight web server. You could
use that image as a base image, copy in your website's HTML files, and package
that into another image. Each time your website changed, you'd need to update
the new image and redeploy all of the containers serving your website. A better
solution is to store the website in a named volume which is attached to each of
your web server containers when they start. To update the website, you just
update the named volume.

For more information about named volumes, see
[Data Volumes](https://docs.docker.com/engine/tutorials/dockervolumes/).

The following table describes options which apply to both bind-mounts and named
volumes in a service:

<table>
  <tr>
    <th>Option</th>
    <th>Required</th>
    <th>Description</th>
  </tr>
  <tr>
    <td><b>types</b></td>
    <td></td>
    <td>
      <p>The type of mount, can be either <tt>volume</tt>, <tt>bind</tt>, or <tt>tmpfs</tt>. Defaults to <tt>volume</tt> if no type is specified.
      <ul>
        <li><tt>volume</tt>: mounts a [managed volume](volume_create.md) into the container.</li>
        <li><tt>bind</tt>: bind-mounts a directory or file from the host into the container.</li>
        <li><tt>tmpfs</tt>: mount a tmpfs in the container</li>
      </ul></p>
    </td>
  </tr>
  <tr>
    <td><b>src</b> or <b>source</b></td>
    <td>for <tt>type=bind</tt> only></td>
    <td>
      <ul>
        <li>
         <tt>type=volume</tt>: <tt>src</tt> is an optional way to specify the name of the volume (for example, <tt>src=my-volume</tt>).
          If the named volume does not exist, it is automatically created. If no <tt>src</tt> is specified, the volume is
          assigned a random name which is guaranteed to be unique on the host, but may not be unique cluster-wide.
          A randomly-named volume has the same lifecycle as its container and is destroyed when the <i>container</i>
          is destroyed (which is upon <tt>service update</tt>, or when scaling or re-balancing the service)
        </li>
        <li>
          <tt>type=bind</tt>: <tt>src</tt> is required, and specifies an absolute path to the file or directory to bind-mount
          (for example, <tt>src=/path/on/host/</tt>). An error is produced if the file or directory does not exist.
        </li>
        <li>
          <tt>type=tmpfs</tt>: <tt>src</tt> is not supported.
        </li>
      </ul>
    </td>
  </tr>
  <tr>
    <td><p><b>dst</b> or <b>destination</b> or <b>target</b></p></td>
    <td>yes</td>
    <td>
      <p>Mount path inside the container, for example <tt>/some/path/in/container/</tt>.
      If the path does not exist in the container's filesystem, the Engine creates
      a directory at the specified location before mounting the volume or bind-mount.</p>
    </td>
  </tr>
  <tr>
    <td><p><b>readonly</b> or <b>ro</b></p></td>
    <td></td>
    <td>
      <p>The Engine mounts binds and volumes <tt>read-write</tt> unless <tt>readonly</tt> option
      is given when mounting the bind or volume.
      <ul>
        <li><tt>true</tt> or <tt>1</tt> or no value: Mounts the bind or volume read-only.</li>
        <li><tt>false</tt> or <tt>0</tt>: Mounts the bind or volume read-write.</li>
      </ul></p>
    </td>
  </tr>
  <tr>
    <td><b>consistency</b></td>
    <td></td>
    <td>
      <p>The consistency requirements for the mount; one of
         <ul>
           <li><tt>default</tt>: Equivalent to <tt>consistent</tt>.</li>
           <li><tt>consistent</tt>: Full consistency.  The container runtime and the host maintain an identical view of the mount at all times.</li>
           <li><tt>cached</tt>: The host's view of the mount is authoritative.  There may be delays before updates made on the host are visible within a container.</li>
           <li><tt>delegated</tt>: The container runtime's view of the mount is authoritative.  There may be delays before updates made in a container are are visible on the host.</li>
        </ul>
     </p>
    </td>
  </tr>
</table>

#### Bind Propagation

Bind propagation refers to whether or not mounts created within a given
bind-mount or named volume can be propagated to replicas of that mount. Consider
a mount point `/mnt`, which is also mounted on `/tmp`. The propation settings
control whether a mount on `/tmp/a` would also be available on `/mnt/a`. Each
propagation setting has a recursive counterpoint. In the case of recursion,
consider that `/tmp/a` is also mounted as `/foo`. The propagation settings
control whether `/mnt/a` and/or `/tmp/a` would exist.

The `bind-propagation` option defaults to `rprivate` for both bind-mounts and
volume mounts, and is only configurable for bind-mounts. In other words, named
volumes do not support bind propagation.

- **`shared`**: Sub-mounts of the original mount are exposed to replica mounts,
                and sub-mounts of replica mounts are also propagated to the
                original mount.
- **`slave`**: similar to a shared mount, but only in one direction. If the
               original mount exposes a sub-mount, the replica mount can see it.
               However, if the replica mount exposes a sub-mount, the original
               mount cannot see it.
- **`private`**: The mount is private. Sub-mounts within it are not exposed to
                 replica mounts, and sub-mounts of replica mounts are not
                 exposed to the original mount.
- **`rshared`**: The same as shared, but the propagation also extends to and from
                 mount points nested within any of the original or replica mount
                 points.
- **`rslave`**: The same as `slave`, but the propagation also extends to and from
                 mount points nested within any of the original or replica mount
                 points.
- **`rprivate`**: The default. The same as `private`, meaning that no mount points
                  anywhere within the original or replica mount points propagate
                  in either direction.

For more information about bind propagation, see the
[Linux kernel documentation for shared subtree](https://www.kernel.org/doc/Documentation/filesystems/sharedsubtree.txt).

#### Options for Named Volumes

The following options can only be used for named volumes (`type=volume`);


<table>
  <tr>
    <th>Option</th>
    <th>Description</th>
  </tr>
  <tr>
    <td><b>volume-driver</b></td>
    <td>
      <p>Name of the volume-driver plugin to use for the volume. Defaults to
      <tt>"local"</tt>, to use the local volume driver to create the volume if the
      volume does not exist.</p>
    </td>
  </tr>
  <tr>
    <td><b>volume-label</b></td>
    <td>
      One or more custom metadata ("labels") to apply to the volume upon
      creation. For example,
      `volume-label=mylabel=hello-world,my-other-label=hello-mars`. For more
      information about labels, refer to
      <a href="https://docs.docker.com/engine/userguide/labels-custom-metadata/">apply custom metadata</a>.
    </td>
  </tr>
  <tr>
    <td><b>volume-nocopy</b></td>
    <td>
      By default, if you attach an empty volume to a container, and files or
      directories already existed at the mount-path in the container (<tt>dst</tt>),
      the Engine copies those files and directories into the volume, allowing
      the host to access them. Set `volume-nocopy` to disables copying files
      from the container's filesystem to the volume and mount the empty volume.

      A value is optional:

      <ul>
        <li><tt>true</tt> or <tt>1</tt>: Default if you do not provide a value. Disables copying.</li>
        <li><tt>false</tt> or <tt>0</tt>: Enables copying.</li>
      </ul>
    </td>
  </tr>
  <tr>
    <td><b>volume-opt</b></td>
    <td>
      Options specific to a given volume driver, which will be passed to the
      driver when creating the volume. Options are provided as a comma-separated
      list of key/value pairs, for example,
      <tt>volume-opt=some-option=some-value,volume-opt=some-other-option=some-other-value</tt>.
      For available options for a given driver, refer to that driver's
      documentation.
    </td>
  </tr>
</table>


#### Options for tmpfs

The following options can only be used for tmpfs mounts (`type=tmpfs`);


<table>
  <tr>
    <th>Option</th>
    <th>Description</th>
  </tr>
  <tr>
    <td><b>tmpfs-size</b></td>
    <td>Size of the tmpfs mount in bytes. Unlimited by default in Linux.</td>
  </tr>
  <tr>
    <td><b>tmpfs-mode</b></td>
    <td>File mode of the tmpfs in octal. (e.g. <tt>"700"</tt> or <tt>"0700"</tt>.) Defaults to <tt>"1777"</tt> in Linux.</td>
  </tr>
</table>


#### Differences between "--mount" and "--volume"

The `--mount` flag supports most options that are supported by the `-v`
or `--volume` flag for `docker run`, with some important exceptions:

- The `--mount` flag allows you to specify a volume driver and volume driver
  options *per volume*, without creating the volumes in advance. In contrast,
  `docker run` allows you to specify a single volume driver which is shared
  by all volumes, using the `--volume-driver` flag.

- The `--mount` flag allows you to specify custom metadata ("labels") for a volume,
  before the volume is created.

- When you use `--mount` with `type=bind`, the host-path must refer to an *existing*
  path on the host. The path will not be created for you and the service will fail
  with an error if the path does not exist.

- The `--mount` flag does not allow you to relabel a volume with `Z` or `z` flags,
  which are used for `selinux` labeling.

#### Create a service using a named volume

The following example creates a service that uses a named volume:

```bash
$ docker service create \
  --name my-service \
  --replicas 3 \
  --mount type=volume,source=my-volume,destination=/path/in/container,volume-label="color=red",volume-label="shape=round" \
  nginx:alpine
```

For each replica of the service, the engine requests a volume named "my-volume"
from the default ("local") volume driver where the task is deployed. If the
volume does not exist, the engine creates a new volume and applies the "color"
and "shape" labels.

When the task is started, the volume is mounted on `/path/in/container/` inside
the container.

Be aware that the default ("local") volume is a locally scoped volume driver.
This means that depending on where a task is deployed, either that task gets a
*new* volume named "my-volume", or shares the same "my-volume" with other tasks
of the same service. Multiple containers writing to a single shared volume can
cause data corruption if the software running inside the container is not
designed to handle concurrent processes writing to the same location. Also take
into account that containers can be re-scheduled by the Swarm orchestrator and
be deployed on a different node.

#### Create a service that uses an anonymous volume

The following command creates a service with three replicas with an anonymous
volume on `/path/in/container`:

```bash
$ docker service create \
  --name my-service \
  --replicas 3 \
  --mount type=volume,destination=/path/in/container \
  nginx:alpine
```

In this example, no name (`source`) is specified for the volume, so a new volume
is created for each task. This guarantees that each task gets its own volume,
and volumes are not shared between tasks. Anonymous volumes are removed after
the task using them is complete.

#### Create a service that uses a bind-mounted host directory

The following example bind-mounts a host directory at `/path/in/container` in
the containers backing the service:

```bash
$ docker service create \
  --name my-service \
  --mount type=bind,source=/path/on/host,destination=/path/in/container \
  nginx:alpine
```

### Set service mode (--mode)

The service mode determines whether this is a _replicated_ service or a _global_
service. A replicated service runs as many tasks as specified, while a global
service runs on each active node in the swarm.

The following command creates a global service:

```bash
$ docker service create \
 --name redis_2 \
 --mode global \
 redis:3.0.6
```

### Specify service constraints (--constraint)

You can limit the set of nodes where a task can be scheduled by defining
constraint expressions. Multiple constraints find nodes that satisfy every
expression (AND match). Constraints can match node or Docker Engine labels as
follows:


<table>
  <tr>
    <th>node attribute</th>
    <th>matches</th>
    <th>example</th>
  </tr>
  <tr>
    <td><tt>node.id</tt></td>
    <td>Node ID</td>
    <td><tt>node.id == 2ivku8v2gvtg4</tt></td>
  </tr>
  <tr>
    <td><tt>node.hostname</tt></td>
    <td>Node hostname</td>
    <td><tt>node.hostname != node-2</tt></td>
  </tr>
  <tr>
    <td><tt>node.role</tt></td>
    <td>Node role</td>
    <td><tt>node.role == manager</tt></td>
  </tr>
  <tr>
    <td><tt>node.labels</tt></td>
    <td>user defined node labels</td>
    <td><tt>node.labels.security == high</tt></td>
  </tr>
  <tr>
    <td><tt>engine.labels</tt></td>
    <td>Docker Engine's labels</td>
    <td><tt>engine.labels.operatingsystem == ubuntu 14.04</tt></td>
  </tr>
</table>


`engine.labels` apply to Docker Engine labels like operating system,
drivers, etc. Swarm administrators add `node.labels` for operational purposes by
using the [`docker node update`](node_update.md) command.

For example, the following limits tasks for the redis service to nodes where the
node type label equals queue:

```bash
$ docker service create \
  --name redis_2 \
  --constraint 'node.labels.type == queue' \
  redis:3.0.6
```

### Specify service placement preferences (--placement-pref)

You can set up the service to divide tasks evenly over different categories of
nodes. One example of where this can be useful is to balance tasks over a set
of datacenters or availability zones. The example below illustrates this:

```bash
$ docker service create \
  --replicas 9 \
  --name redis_2 \
  --placement-pref 'spread=node.labels.datacenter' \
  redis:3.0.6
```

This uses `--placement-pref` with a `spread` strategy (currently the only
supported strategy) to spread tasks evenly over the values of the `datacenter`
node label. In this example, we assume that every node has a `datacenter` node
label attached to it. If there are three different values of this label among
nodes in the swarm, one third of the tasks will be placed on the nodes
associated with each value. This is true even if there are more nodes with one
value than another. For example, consider the following set of nodes:

- Three nodes with `node.labels.datacenter=east`
- Two nodes with `node.labels.datacenter=south`
- One node with `node.labels.datacenter=west`

Since we are spreading over the values of the `datacenter` label and the
service has 9 replicas, 3 replicas will end up in each datacenter. There are
three nodes associated with the value `east`, so each one will get one of the
three replicas reserved for this value. There are two nodes with the value
`south`, and the three replicas for this value will be divided between them,
with one receiving two replicas and another receiving just one. Finally, `west`
has a single node that will get all three replicas reserved for `west`.

If the nodes in one category (for example, those with
`node.labels.datacenter=south`) can't handle their fair share of tasks due to
constraints or resource limitations, the extra tasks will be assigned to other
nodes instead, if possible.

Both engine labels and node labels are supported by placement preferences. The
example above uses a node label, because the label is referenced with
`node.labels.datacenter`. To spread over the values of an engine label, use
`--placement-pref spread=engine.labels.<labelname>`.

It is possible to add multiple placement preferences to a service. This
establishes a hierarchy of preferences, so that tasks are first divided over
one category, and then further divided over additional categories. One example
of where this may be useful is dividing tasks fairly between datacenters, and
then splitting the tasks within each datacenter over a choice of racks. To add
multiple placement preferences, specify the `--placement-pref` flag multiple
times. The order is significant, and the placement preferences will be applied
in the order given when making scheduling decisions.

The following example sets up a service with multiple placement preferences.
Tasks are spread first over the various datacenters, and then over racks
(as indicated by the respective labels):

```bash
$ docker service create \
  --replicas 9 \
  --name redis_2 \
  --placement-pref 'spread=node.labels.datacenter' \
  --placement-pref 'spread=node.labels.rack' \
  redis:3.0.6
```

When updating a service with `docker service update`, `--placement-pref-add`
appends a new placement preference after all existing placement preferences.
`--placement-pref-rm` removes an existing placement preference that matches the
argument.

### Attach a service to an existing network (--network)

You can use overlay networks to connect one or more services within the swarm.

First, create an overlay network on a manager node the docker network create
command:

```bash
$ docker network create --driver overlay my-network

etjpu59cykrptrgw0z0hk5snf
```

After you create an overlay network in swarm mode, all manager nodes have
access to the network.

When you create a service and pass the --network flag to attach the service to
the overlay network:

```bash
$ docker service create \
  --replicas 3 \
  --network my-network \
  --name my-web \
  nginx

716thylsndqma81j6kkkb5aus
```

The swarm extends my-network to each node running the service.

Containers on the same network can access each other using
[service discovery](https://docs.docker.com/engine/swarm/networking/#use-swarm-mode-service-discovery).

### Publish service ports externally to the swarm (-p, --publish)

You can publish service ports to make them available externally to the swarm
using the `--publish` flag:

```bash
$ docker service create --publish <TARGET-PORT>:<SERVICE-PORT> nginx
```

For example:

```bash
$ docker service create --name my_web --replicas 3 --publish 8080:80 nginx
```

When you publish a service port, the swarm routing mesh makes the service
accessible at the target port on every node regardless if there is a task for
the service running on the node. For more information refer to
[Use swarm mode routing mesh](https://docs.docker.com/engine/swarm/ingress/).

### Publish a port for TCP only or UDP only

By default, when you publish a port, it is a TCP port. You can
specifically publish a UDP port instead of or in addition to a TCP port. When
you publish both TCP and UDP ports, Docker 1.12.2 and earlier require you to
add the suffix `/tcp` for TCP ports. Otherwise it is optional.

#### TCP only

The following two commands are equivalent.

```bash
$ docker service create --name dns-cache -p 53:53 dns-cache

$ docker service create --name dns-cache -p 53:53/tcp dns-cache
```

#### TCP and UDP

```bash
$ docker service create --name dns-cache -p 53:53/tcp -p 53:53/udp dns-cache
```

#### UDP only

```bash
$ docker service create --name dns-cache -p 53:53/udp dns-cache
```

### Create services using templates

You can use templates for some flags of `service create`, using the syntax
provided by the Go's [text/template](http://golang.org/pkg/text/template/) package.

The supported flags are the following :

- `--hostname`
- `--mount`
- `--env`

Valid placeholders for the Go template are listed below:


<table>
  <tr>
    <th>Placeholder</th>
    <th>Description</th>
  </tr>
  <tr>
    <td><tt>.Service.ID</tt></td>
    <td>Service ID</td>
  </tr>
  <tr>
    <td><tt>.Service.Name</tt></td>
    <td>Service name</td>
  </tr>
  <tr>
    <td><tt>.Service.Labels</tt></td>
    <td>Service labels</td>
  </tr>
  <tr>
    <td><tt>.Node.ID</tt></td>
    <td>Node ID</td>
  </tr>
  <tr>
    <td><tt>.Task.ID</tt></td>
    <td>Task ID</td>
  </tr>
  <tr>
    <td><tt>.Task.Name</tt></td>
    <td>Task name</td>
  </tr>
  <tr>
    <td><tt>.Task.Slot</tt></td>
    <td>Task slot</td>
  </tr>
</table>


#### Template example

In this example, we are going to set the template of the created containers based on the
service's name and the node's ID where it sits.

```bash
$ docker service create --name hosttempl \
                        --hostname="{{.Node.ID}}-{{.Service.Name}}"\
                         busybox top

va8ew30grofhjoychbr6iot8c

$ docker service ps va8ew30grofhjoychbr6iot8c

ID            NAME         IMAGE                                                                                   NODE          DESIRED STATE  CURRENT STATE               ERROR  PORTS
wo41w8hg8qan  hosttempl.1  busybox:latest@sha256:29f5d56d12684887bdfa50dcd29fc31eea4aaf4ad3bec43daf19026a7ce69912  2e7a8a9c4da2  Running        Running about a minute ago

$ docker inspect --format="{{.Config.Hostname}}" hosttempl.1.wo41w8hg8qanxwjwsg4kxpprj

x3ti0erg11rjpg64m75kej2mz-hosttempl
```

## Related commands

* [service inspect](service_inspect.md)
* [service logs](service_logs.md)
* [service ls](service_ls.md)
* [service rm](service_rm.md)
* [service scale](service_scale.md)
* [service ps](service_ps.md)
* [service update](service_update.md)

<style>table tr > td:first-child { white-space: nowrap;}</style>

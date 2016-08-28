# Docker Introspection Mount

The introspection mount is a new feature that allows you to introspect the
metadata about the container from the container itself, via a procfs-like
filesystem.

## Hierarchy

If you enable the introspection mount, following files are created under the mount point:

- `container/id` (scope: `.container.id`): ID string of the container. e.g. `2f3cc2b029e0ca46564d5a5e38772b09947056f3b22b6a114054a468382e872e\n`
- `container/name` (scope: `.container.name`): Name of the container. e.g. `nginx.3.8tc0va0kw59rbbdh5x3iqc3v9\n`
- `container/fullname` (scope: `.container.fullname`): Full name of the container. e.g. `/nginx.3.8tc0va0kw59rbbdh5x3iqc3v9\n`
- `container/labels/{LABELNAME}` (scope: `.container.labels`): Label of the container. e.g. the content of `container/labels/com.docker.swarm.service.name` would be `nginx\n`. Note that all the labels belong to the single `.container.labels` scope.

- `daemon/name` (scope: `.daemon.name`): Hostname of the daemon node. e.g. `host01\n`

For Swarm task containers running on a manager node, following files appear as well:

- `service/id` (scope: `.service.id`): ID string of the service. e.g. `6h7nic7tsv16cfo0qhywj7bsh\n`
- `service/name` (scope: `.service.name`): Name of the service .e.g. `nginx\n`

- `task/id` (scope: `.task.id`): ID string of the task. e.g. `8tc0va0kw59rbbdh5x3iqc3v9\n`
- `task/name` (scope: `.task.name`): Name of the task. e.g. `nginx.3.8tc0va0kw59rbbdh5x3iqc3v9`
- `task/slot` (scope: `.task.slot`): Slot number (1-based index for replicas) of the task. e.g. `1\n`. Please also refer to [the documentation of the Swarmkit](https://github.com/docker/swarmkit/blob/master/design/task_model.md#slot-model). Note that there are cases where a slot may have multiple tasks with the desired state of `RUNNING`.

## Use cases for the introspection mount

Below are some example use cases for the introspection mount.

### Deploying a service that requires the task slot number (e.g. Apache ZooKeeper)

Apache ZooKeeper is a highly available coordination service that is used by
distributed systems such as Hadoop.  A typical configuration file (`zoo.cfg`)
for ZooKeeper would be as follows:

    tickTime=2000
	dataDir=/var/lib/zookeeper
	clientPort=2181
	initLimit=5
	syncLimit=2
	server.1=zoo1:2888:3888
	server.2=zoo2:2888:3888
	server.3=zoo3:2888:3888

ZooKeeper also requires a file named `myid` to be located under `dataDir`.
The content of `myid` is `1\n` for `server.1`, `2` for `server.2`, and so on.

The `task/slot` file under the introspection mount can be used for generating
the `myid` file.

See also [#24110](https://github.com/docker/docker/issues/24110).

### 3rd party job scheduler

A 3rd party job scheduler can be built on a Docker service using the
introspection mount.

For example, the `task/slot` file under the introspection mount can be used for
implementing a scheduler that executes multiple batch jobs in parallel.
(Similar to the `{%}` symbol in the GNU parallel.)

See also [#23843](https://github.com/docker/docker/issues/23843).

### 3rd party orchestration/monitoring tool

A container can send the `container/id` file under the introspection mount to
some 3rd party orchestration/monitoring tool.  Then such a tool take appropriate
action using the ID information.

See also [#7685](https://github.com/docker/docker/pull/7685).

## Using the introspection mount

Create a service with the introspection mount `/foo` with all scopes (`.`):

    $ docker service create --name nginx --replicas 3 --mount type=introspection,dst=/foo,introspection-scope=. nginx
    
Enter a container and read the files under `/foo`:

    $ docker exec -it $(docker ps -q -f label=com.docker.swarm.service.name=nginx | head -n 1) sh
    # find /foo
    /foo
    /foo/container
    /foo/container/name
    /foo/container/labels
    /foo/container/labels/com.docker.swarm.service.id
    /foo/container/labels/com.docker.swarm.node.id
    /foo/container/labels/com.docker.swarm.task.name
    /foo/container/labels/com.docker.swarm.task
    /foo/container/labels/com.docker.swarm.task.id
    /foo/container/labels/com.docker.swarm.service.name
    /foo/container/id
    /foo/container/fullname
    /foo/task
    /foo/task/name
    /foo/task/slot
    /foo/task/id
    /foo/service
    /foo/service/name
    /foo/service/id
    /foo/daemon
    /foo/daemon/name
    # cat /foo/task/slot
    3


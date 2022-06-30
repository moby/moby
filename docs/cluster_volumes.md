Cluster Volumes
===============

Docker Cluster Volumes is a new feature which allows using CSI plugins to
create cluster-aware volumes.

The Container Storage Interface is a platform-agnostic API for storage
providers to write storage plugins which are compatible with many container
orchestrators. By leveraging the CSI, Docker Swarm can provide intelligent,
cluster-aware access to volumes across many supported storage providers.

## Installing a CSI plugin

Docker accesses CSI plugins through the Docker managed plugin system, using the
`docker plugin` command.

If a plugin is available for Docker, it can be installed through the `docker
plugin install` command. Plugins may require configuration specific to the
user's environment, they will ultimately be detected by and work automatically
with Docker once enabled.

Currently, there is no way to automatically deploy a Docker Plugin across all
nodes in a cluster. Therefore, users must ensure the Docker Plugin is installed
on all nodes in the cluster on which it is desired. 

The CSI plugin must be installed on all manager nodes. If a manager node does
not have the CSI plugin installed, a leadership change to that manager nodes
will make Swarm unable to use that driver.

Docker Swarm worker nodes report their active plugins to the Docker Swarm
managers, so it is not necessary to install a plugin on every worker node. The
plugin only needs to be installed on those nodes need access to the volumes
provided by that plugin.

### Multiple Instances of the Same Plugin

In some cases, it may be desirable to run multiple instances of the same
plugin. For example, there may be two different instances of some storage
provider which each need a differently configured plugin.

To run more than one instance of the same plugin, set the `--alias` option when
installing the plugin. This will cause the plugin to take a local name
different from its original name.

Ensure that when using plugin name aliases, the plugin name alias is the same
on every node.

## Creating a Docker CSI Plugin

Most CSI plugins are shipped with configuration specific to Kubernetes. They
are often provided in the form of Helm charts, and installation information may
include Kubernetes-specific steps. Docker CSI Plugins use the same binaries as
those for Kubernetes, but in a different environment and sometimes with
different configuration.

Before following this section, readers should ensure they are acquainted with
the
[Docker Engine managed plugin system](https://docs.docker.com/engine/extend/).
Docker CSI plugins use this system to run.

Docker Plugins consist of a root filesystem and a `config.json`. The root
filesystem can generally be exported from whatever image is built for the
plugin. The `config.json` specifies how the plugin is used. Several
CSI-specific concerns, as well as some general but poorly-documented features,
are outlined here.

### Basic Requirements

Docker CSI plugins are identified with a special interface type. There are two
related interfaces that CSI plugins can expose.

* `docker.csicontroller/1.0` is used for CSI Controller plugins.
* `docker.csinode/1.0` is used for CSI Node plugins.
* Combined plugins should include both interfaces.

Additionally, the interface field of the config.json includes a `socket` field.
This can be set to any value, but the CSI plugin should have its `CSI_ENDPOINT`
environment variable set appropriately.

In the `config.json`, this should be set as such:

```json
  "interface": {
    "types": ["docker.csicontroller/1.0","docker.csinode/1.0"],
    "socket": "my-csi-plugin.sock"
  },
  "env": [
    {
      "name": "CSI_ENDPOINT",
      "value": "/run/docker/plugins/my-csi-plugin.sock"
    }
  ]
```

The CSI specification states that CSI plugins should have
`CAP_SYS_ADMIN` privileges, so this should be set in the `config.json` as
well:

```json
  "linux" : {
    "capabilities": ["CAP_SYS_ADMIN"]
  }
```

### Propagated Mount

In order for the plugin to expose volumes to Swarm, it must publish those
volumes to a Propagated Mount location. This allows a mount to be itself
mounted to a different location in the filesystem. The Docker Plugin system
only allows one Propagated Mount, which is configured as a string representing
the path in the plugin filesystem.

When calling the CSI plugin, Docker Swarm specifies the publish target path,
which is the path in the plugin filesystem that a volume should ultimately be
used from. This is also the path that needs to be specified as the Propagated
Mount for the plugin. This path is hard-coded to be `/data/published` in the
plugin filesystem, and as such, the plugin configuration should list this as
the Propagated Mount:

```json
  "propagatedMount": "/data/published"
```

### Configurable Options

Plugin configurations can specify configurable options for many fields. To
expose a field as configurable, the object including that field should include
a field `Settable`, which is an array of strings specifying the name of
settable fields.

For example, consider a plugin that supports a config file.

```json
  "mounts": [
    {
      "name": "configfile",
      "description": "Config file mounted in from the host filesystem",
      "type": "bind",
      "destination": "/opt/my-csi-plugin/config.yaml",
      "source": "/etc/my-csi-plugin/config.yaml"
    }
  ]
```

This configuration would result in a file located on the host filesystem at
`/etc/my-csi-plugin/config.yaml` being mounted into the plugin filesystem at
`/opt/my-csi-plugin/config.yaml`. However, hard-specifying the source path of
the configuration is undesirable. Instead, the plugin author can put the
`Source` field in the Settable array:

```json
  "mounts": [
    {
      "name": "configfile",
      "description": "Config file mounted in from the host filesystem",
      "type": "bind",
      "destination": "/opt/my-csi-plugin/config.yaml",
      "source": "",
      "settable": ["source"]
    }
  ]
```

When a field is exposed as settable, the user can configure that field when
installing the plugin.

```
$ docker plugin install my-csi-plugin configfile.source="/srv/my-csi-plugin/config.yaml"
```

Or, alternatively, it can be set while the plugin is disabled:

```
$ docker plugin disable my-csi-plugin
$ docker plugin set my-csi-plugin configfile.source="/var/lib/my-csi-plugin/config.yaml"
$ docker plugin enable
```

### Split-Component Plugins

For split-component plugins, users can specify either the
`docker.csicontroller/1.0` or `docker.csinode/1.0` plugin interfaces. Manager
nodes should run plugin instances with the `docker.csicontroller/1.0`
interface, and worker nodes the `docker.csinode/1.0` interface.

Docker does support running two plugins with the same name, nor does it support
specifying different drivers for the node and controller plugins. This means in
a fully split plugin, Swarm will be unable to schedule volumes to manager
nodes.

If it is desired to run a split-component plugin such that the Volumes managed
by that plugin are accessible to Tasks on the manager node, the user will need
to build the plugin such that some proxy or multiplexer provides the illusion
of combined components to the manager through one socket, and ensure the plugin
reports both interface types.

## Using Cluster Volumes

### Create a Cluster Volume

Creating a Cluster Volume is done with the same `docker volume` commands as any
other Volume. To create a Cluster Volume, one needs to do both of things:

* Specify a CSI-capable driver with the `--driver` or `-d` option.
* Use any one of the cluster-specific `docker volume create` flags.

For example, to create a Cluster Volume called `my-volume` with the
`democratic-csi` Volume Driver, one might use this command:

```bash
docker volume create \
  --driver democratic-csi \
  --type mount \
  --sharing all \
  --scope multi \
  --limit-bytes 10G \
  --required-bytes 1G \
  my-volume
```

### List Cluster Volumes

Cluster Volumes will be listed along with other volumes when doing
`docker volume ls`. However, if users want to see only Cluster Volumes, and
with cluster-specific information, the flag `--cluster` can be specified:

```
$ docker volume ls --cluster
VOLUME NAME   GROUP     DRIVER    AVAILABILITY   STATUS
volume1       group1    driver1   active         pending creation
volume2       group1    driver1   pause          created
volume3       group2    driver2   active         in use (1 node)
volume4       group2    driver2   active         in use (2 nodes)
```

### Deploying a Service

Cluster Volumes are only compatible with Docker Services, not plain Docker
Containers.

In Docker Services, a Cluster Volume is used the same way any other volume
would be used. The `type` should be set to `csi`. For example, to create a
Service that uses `my-volume` created above, one would execute a command like:

```bash
docker service create \
  --name my-service \
  --mount type=csi,src=my-volume,dst=/srv/www \
  nginx:alpine
```

When scheduling Services which use Cluster Volumes, Docker Swarm uses the
volume's information and state to make decisions about Task placement.

For example, the Service will be constrained to run only on nodes on which the
volume is available. If the volume is configured with `scope=single`, meaning
it can only be used on one node in the cluster at a time, then all Tasks for
that Service will be scheduled to that same node. If that node changes for some
reason, like a node failure, then the Tasks will be rescheduled to the new
node automatically, without user input.

If the Cluster Volume is accessible only on some set of nodes at the same time,
and not the whole cluster, then Docker Swarm will only schedule the Service to
those nodes as reported by the plugin.

### Using Volume Groups

It is frequently desirable that a Service use any available volume out of an
interchangeable set. To accomplish this in the most simple and straightforward
manner possible, Cluster Volumes use the concept of a volume "Group".

The Volume Group is a field, somewhat like a special label, which is used to
instruct Swarm that a given volume is interchangeable with every other volume
of the same Group. When creating a Cluster Volume, the Group can be specified
by using the `--group` flag.

To use a Cluster Volume by Group instead of by Name, the mount `src` option is
prefixed with `group:`, followed by the group name. For example:

```
--mount type=csi,src=group:my-group,dst=/srv/www
```

This instructs Docker Swarm that any Volume with the Group `my-group` can be
used to satisfy the mounts.

Volumes in a Group do not need to be identical, but they must be
interchangeable. These caveats should be kept in mind when using Groups:

* No Service ever gets the monopoly on a Cluster Volume. If several Services
  use the same Group, then the Cluster Volumes in that Group can be used with
  any of those Services at any time. Just because a particular Volume was used
  by a particular Service at one point does not mean it won't be used by a
  different Service later.
* Volumes in a group can have different configurations, but all of those
  configurations must be compatible with the Service. For example, if some of
  the Volumes in a group have `sharing=readonly`, then the Service must be
  capable of using the volume in read-only mode.
* Volumes in a Group are created statically ahead of time, not dynamically
  as-needed. This means that the user must ensure a sufficient number of
  Volumes belong to the desired Group to support the needs of the Service.

### Taking Cluster Volumes Offline

For various reasons, users may wish to take a particular Cluster Volume
offline, such that is not actively used by Services. To facilitate this,
Cluster Volumes have an `availability` option similar to Docker Swarm nodes.

Cluster Volume availability can be one of three states:

* `active` - Default. Volume can be used as normal.
* `pause` - The volume will not be used for new Services, but existing Tasks
  using the volume will not be stopped.
* `drain` - The volume will not be used for new Services, and any running Tasks
  using the volume will be stopped and rescheduled.

A Volume can only be removed from the cluster entirely if its availability is
set to `drain`, and it has been fully unpublished from all nodes.

## Unsupported Features

The CSI Spec allows for a large number of features which Cluster Volumes in
this initial implementation do not support. Most notably, Cluster Volumes do
not support snapshots, cloning, or volume expansion.

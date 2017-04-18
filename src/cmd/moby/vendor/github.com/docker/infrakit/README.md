InfraKit
========

[![Circle CI](https://circleci.com/gh/docker/infrakit.png?style=shield&circle-token=50d2063f283f98b7d94746416c979af3102275b5)](https://circleci.com/gh/docker/infrakit)
[![Go Report Card](https://goreportcard.com/badge/github.com/docker/infrakit)](https://goreportcard.com/report/github.com/docker/infrakit)
[![codecov.io](https://codecov.io/github/docker/infrakit/coverage.svg?branch=master&token=z08ZKeIJfA)](https://codecov.io/github/docker/infrakit?branch=master)

_InfraKit_ is a toolkit for creating and managing declarative, self-healing infrastructure.
It breaks infrastructure automation down into simple, pluggable components. These components work together to actively
ensure the infrastructure state matches the user's specifications.
Although _InfraKit_ emphasizes primitives for building self-healing infrastructure, it also can be used passively like
conventional tools.

To get started, try the [tutorial](docs/tutorial.md).

### Who InfraKit is for

_InfraKit_ is designed to support setup and management of base infrastructure.  For example, it can help you manage a
system like a cluster or container orchestrator, ideally relieving you of building custom release and maintenance tools.
As a result, it is a low-level tool intended to be used by infrastructure operators directly or indirectly
(as a toolkit) through a higher-level tool.  Since _InfraKit_ is pluggable, it allows you to manage resources in diverse
environments while using shared components and consistent interfaces.

## Plugins
_InfraKit_ makes extensive use of _Plugins_ to manage arbitrary systems in diverse environments, which can be composed
to meet different needs.

See the [plugins](docs/plugins) documentation for more details.


## Building
### Your Environment

Make sure you check out the project following a convention for building Go projects. For example,

```shell

# Install Go - https://golang.org/dl/
# Assuming your go compiler is in /usr/local/go
export PATH=/usr/local/go/bin:$PATH

# Your dev environment
mkdir -p ~/go
export GOPATH=!$
export PATH=$GOPATH/bin:$PATH

mkdir -p ~/go/src/github.com/docker
cd !$
git clone git@github.com:docker/infrakit.git
cd infrakit
```

We recommended go version 1.7.1 or greater for all platforms.

Also install a few build tools:
```shell
make get-tools
```

### Running tests
```shell
$ make ci
```

### Binaries
```shell
$ make binaries
```
Executables will be placed in the `./build` directory.
This will produce binaries for tools and several reference Plugin implementations:
  + [`infrakit`](cmd/cli/README.md): a command line interface to interact with plugins
  + [`infrakit-group-default`](cmd/group/README.md): the default [Group plugin](./pkg/spi/group)
  + [`infrakit-instance-file`](examples/instance/file): an Instance plugin using dummy files to represent instances
  + [`infrakit-instance-terraform`](examples/instance/terraform):
    an Instance plugin integrating [Terraform](https://www.terraform.io)
  + [`infrakit-instance-vagrant`](examples/instance/vagrant):
    an Instance plugin using [Vagrant](https://www.vagrantup.com/)
  + [`infrakit-instance-maas`](examples/instance/maas):
    an Instance plugin using [MaaS](https://maas.io)
  + [`infrakit-flavor-vanilla`](examples/flavor/vanilla):
    a Flavor plugin for plain vanilla set up with user data and labels
  + [`infrakit-flavor-zookeeper`](examples/flavor/zookeeper):
    a Flavor plugin for [Apache ZooKeeper](https://zookeeper.apache.org/) ensemble members
  + [`infrakit-flavor-swarm`](examples/flavor/swarm):
    a Flavor plugin for Docker in [Swarm mode](https://docs.docker.com/engine/swarm/).

All provided binaries have a `help` sub-command to get usage and a `version` sub-command to identify the build revision.


# Design

## Configuration
_InfraKit_ uses JSON for configuration because it is composable and a widely accepted format for many
infrastructure SDKs and tools.  Since the system is highly component-driven, our JSON format follows
simple patterns to support the composition of components.

A common pattern for a JSON object looks like this:

```json
{
   "SomeKey": "ValueForTheKey",
   "Properties": {
   }
}
```

There is only one `Properties` field in this JSON and its value is a JSON object. The opaque
JSON value for `Properties` is decoded via the Go `Spec` struct defined within the package of the plugin --
for example -- [`vanilla.Spec`](pkg/plugin/flavor/vanilla/flavor.go).

The JSON above is a _value_, but the type of the value belongs outside the structure.  For example, the
default Group [Spec](pkg/plugin/group/types/types.go) is composed of an Instance plugin, a Flavor plugin, and an
Allocation:

```json
{
  "ID": "name-of-the-group",
  "Properties": {
    "Allocation": {
    },
    "Instance": {
      "Plugin": "name-of-the-instance-plugin",
      "Properties": {
      }
    },
    "Flavor": {
      "Plugin": "name-of-the-flavor-plugin",
      "Properties": {
      }
    }
  }
}
```
The group's Spec has `Instance` and `Flavor` fields which are used to indicate the type, and the value of the
fields follow the pattern of `<some_key>` and `Properties` as shown above.

The `Allocation` determines how the Group is managed.  Allocation has two properties:
  - `Size`: an integer for the number of instances to maintain in the Group
  - `LogicalIDs`: a list of string identifiers, one will be associated with each Instance

Exactly one of these fields must be set, which defines whether the Group is treated as 'cattle' (`Size`) or 'pets'
(`LogicalIDs`).  It is up to the Instance and Flavor plugins to determine how to use `LogicalID` values.

As an example, if you wanted to manage a Group of NGINX servers, you could
write a custom Group plugin for ultimate customization.  The most concise configuration looks something like this:

```json
{
  "ID": "nginx",
  "Plugin": "my-nginx-group-plugin",
  "Properties": {
    "port": 8080
  }
}
````

However, you would likely prefer to use the default Group plugin and implement a Flavor plugin to focus on
application-specific behavior.  This gives you immediate support for any infrastructure that has an Instance plugin.
Your resulting configuration might look something like this:

```json
{
  "ID": "nginx",
  "Plugin": "group",
  "Properties": {
    "Allocation": {
      "Size": 10
    },
    "Instance": {
      "Plugin": "aws",
      "Properties": {
        "region": "us-west-2",
        "ami": "ami-123456"
      }
    },
    "Flavor": {
      "Plugin": "nginx",
      "Properties": {
        "port": 8080
      }
    }
  }
}
```

Once the configuration is ready, you will tell a Group plugin to
  + watch it
  + update it
  + destroy it

Watching the group as specified in the configuration means that the Group plugin will create
the instances if they don't already exist.  New instances will be created if for any reason
existing instances have disappeared such that the state doesn't match your specifications.

Updating the group tells the Group plugin that your configuration may have changed.  It will
then determine the changes necessary to ensure the state of the infrastructure matches the new
specification.

## Docs

Additional documentation can be found [here](docs).

## Reporting security issues

The maintainers take security seriously. If you discover a security issue,
please bring it to their attention right away!

Please **DO NOT** file a public issue, instead send your report privately to
[security@docker.com](mailto:security@docker.com).

Security reports are greatly appreciated and we will publicly thank you for it.
We also like to send gifts—if you're into Docker schwag, make sure to let
us know. We currently do not offer a paid security bounty program, but are not
ruling it out in the future.


## Design goals

_InfraKit_ is currently focused on supporting setup and management of base infrastructure, such as a cluster
orchestrator.  The image below illustrates an architecture we are working towards supporting - a Docker cluster in Swarm
mode.

![arch image](docs/images/arch.png)

This configuration co-locates _InfraKit_ with Swarm manager nodes and offers high availability of _InfraKit_ itself and
Swarm managers (using attached storage).  _InfraKit_ is shown managing two groups - managers and workers that will be
continuously monitored, and may be modified with rolling updates.

Countless configurations are possible with _InfraKit_, but we believe achieving support for this configuration will
enable a large number of real-world use cases.

## Copyright and license

Copyright © 2016 Docker, Inc. All rights reserved. Released under the Apache 2.0
license. See [LICENSE](LICENSE) for the full license text.

# Docker Stacks and Distributed Application Bundles

## Overview

Docker Stacks and Distributed Application Bundles are experimental features
introduced in Docker 1.12 and Docker Compose 1.8, alongside the concept of
swarm mode, and Nodes and Services in the Engine API.

A Dockerfile can be built into an image, and containers can be created from
that image. Similarly, a docker-compose.yml can be built into a **distributed
application bundle**, and **stacks** can be created from that bundle. In that
sense, the bundle is a multi-services distributable image format.

As of Docker 1.12 and Compose 1.8, the features are experimental. Neither
Docker Engine nor the Docker Registry supports distribution of bundles.

## Producing a bundle

The easiest way to produce a bundle is to generate it using `docker-compose`
from an existing `docker-compose.yml`. Of course, that's just *one* possible way
to proceed, in the same way that `docker build` isn't the only way to produce a
Docker image.

From `docker-compose`:

```bash
$ docker-compose bundle
WARNING: Unsupported key 'network_mode' in services.nsqd - ignoring
WARNING: Unsupported key 'links' in services.nsqd - ignoring
WARNING: Unsupported key 'volumes' in services.nsqd - ignoring
[...]
Wrote bundle to vossibility-stack.dab
```

## Creating a stack from a bundle

A stack is created using the `docker deploy` command:

```bash
$ docker deploy --help
Usage:  docker deploy [OPTIONS] STACK

Deploy a new stack or update an existing stack

Aliases:
  deploy, up

Options:
      --bundle-file string    Path to a Distributed Application Bundle file
  -c, --compose-file string   Path to a Compose file
      --help                  Print usage
      --with-registry-auth    Send registry authentication details to Swarm agents

```

Let's deploy the stack created before:

```bash
$ docker deploy --bundle-file vossibility-stack.dab vossibility-stack
Loading bundle from vossibility-stack.dab
Creating service vossibility-stack_elasticsearch
Creating service vossibility-stack_kibana
Creating service vossibility-stack_logstash
Creating service vossibility-stack_lookupd
Creating service vossibility-stack_nsqd
Creating service vossibility-stack_vossibility-collector
```

We can verify that services were correctly created:

```bash
$ docker service ls
ID            NAME                                     MODE         REPLICAS    IMAGE
29bv0vnlm903  vossibility-stack_lookupd                replicated   1/1         nsqio/nsq@sha256:eeba05599f31eba418e96e71e0984c3dc96963ceb66924dd37a47bf7ce18a662
4awt47624qwh  vossibility-stack_nsqd                   replicated   1/1         nsqio/nsq@sha256:eeba05599f31eba418e96e71e0984c3dc96963ceb66924dd37a47bf7ce18a662
4tjx9biia6fs  vossibility-stack_elasticsearch          replicated   1/1         elasticsearch@sha256:12ac7c6af55d001f71800b83ba91a04f716e58d82e748fa6e5a7359eed2301aa
7563uuzr9eys  vossibility-stack_kibana                 replicated   1/1         kibana@sha256:6995a2d25709a62694a937b8a529ff36da92ebee74bafd7bf00e6caf6db2eb03
9gc5m4met4he  vossibility-stack_logstash               replicated   1/1         logstash@sha256:2dc8bddd1bb4a5a34e8ebaf73749f6413c101b2edef6617f2f7713926d2141fe
axqh55ipl40h  vossibility-stack_vossibility-collector  replicated   1/1         icecrime/vossibility-collector@sha256:f03f2977203ba6253988c18d04061c5ec7aab46bca9dfd89a9a1fa4500989fba
```

## Managing stacks

Stacks are managed using the `docker stack` command:

```bash
# docker stack --help

Usage:  docker stack COMMAND

Manage Docker stacks

Options:
      --help   Print usage

Commands:
  deploy      Deploy a new stack or update an existing stack
  ls          List stacks
  ps          List the tasks in the stack
  rm          Remove the stack
  services    List the services in the stack

Run 'docker stack COMMAND --help' for more information on a command.
```

## Bundle file format

Distributed application bundles are described in a JSON format. When bundles
are persisted as files, the file extension is `.dab` (Docker 1.12RC2 tools use
`.dsb` for the file extension—this will be updated in the next release client).

A bundle has two top-level fields: `version` and `services`. The version used
by Docker 1.12 and later tools is `0.1`.

`services` in the bundle are the services that comprise the app. They
correspond to the new `Service` object introduced in the 1.12 Docker Engine API.

A service has the following fields:

<dl>
    <dt>
        Image (required) <code>string</code>
    </dt>
    <dd>
        The image that the service will run. Docker images should be referenced
        with full content hash to fully specify the deployment artifact for the
        service. Example:
        <code>postgres@sha256:f76245b04ddbcebab5bb6c28e76947f49222c99fec4aadb0bb
        1c24821a 9e83ef</code>
    </dd>
    <dt>
        Command <code>[]string</code>
    </dt>
    <dd>
        Command to run in service containers.
    </dd>
    <dt>
        Args <code>[]string</code>
    </dt>
    <dd>
        Arguments passed to the service containers.
    </dd>
    <dt>
        Env <code>[]string</code>
    </dt>
    <dd>
        Environment variables.
    </dd>
    <dt>
        Labels <code>map[string]string</code>
    </dt>
    <dd>
        Labels used for setting meta data on services.
    </dd>
    <dt>
        Ports <code>[]Port</code>
    </dt>
    <dd>
        Service ports (composed of <code>Port</code> (<code>int</code>) and
        <code>Protocol</code> (<code>string</code>). A service description can
        only specify the container port to be exposed. These ports can be
        mapped on runtime hosts at the operator's discretion.
    </dd>
    <dt>
        WorkingDir <code>string</code>
    </dt>
    <dd>
        Working directory inside the service containers.
    </dd>
    <dt>
        User <code>string</code>
    </dt>
    <dd>
        Username or UID (format: <code>&lt;name|uid&gt;[:&lt;group|gid&gt;]</code>).
    </dd>
    <dt>
        Networks <code>[]string</code>
    </dt>
    <dd>
        Networks that the service containers should be connected to. An entity
        deploying a bundle should create networks as needed.
    </dd>
</dl>

The following is an example of bundlefile with two services:

```json
{
	"Version": "0.1",
	"Services": {
		"redis": {
			"Image": "redis@sha256:4b24131101fa0117bcaa18ac37055fffd9176aa1a240392bb8ea85e0be50f2ce",
			"Networks": ["default"]
		},
		"web": {
			"Image": "dockercloud/hello-world@sha256:fe79a2cfbd17eefc344fb8419420808df95a1e22d93b7f621a7399fd1e9dca1d",
			"Networks": ["default"],
			"User": "web"
		}
	}
}
```

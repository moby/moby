# Docker Stacks

## Overview

Docker Stacks are an experimental feature introduced in Docker 1.12, alongside
the new concepts of Swarms and Services inside the Engine.

A Dockerfile can be built into an image, and containers can be created from that
image. Similarly, a docker-compose.yml can be built into a **bundle**, and
**stacks** can be created from that bundle. In that sense, the bundle is a
multi-services distributable image format.

As of 1.12, the feature is introduced as experimental, and Docker Engine doesn't
support distribution of bundles.

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
    Wrote bundle to vossibility-stack.dsb
    ```

## Creating a stack from a bundle

A stack is created using the `docker deploy` command:

    ```bash
    # docker deploy --help

    Usage:  docker deploy [OPTIONS] STACK

    Create and update a stack

    Options:
      -f, --bundle string   Path to a bundle (Default: STACK.dsb)
          --help            Print usage
    ```

Let's deploy the stack created before:

    ```bash
    # docker deploy vossibility-stack
    Loading bundle from vossibility-stack.dsb
    Creating service vossibility-stack_elasticsearch
    Creating service vossibility-stack_kibana
    Creating service vossibility-stack_logstash
    Creating service vossibility-stack_lookupd
    Creating service vossibility-stack_nsqd
    Creating service vossibility-stack_vossibility-collector
    ```

We can verify that services were correctly created:

    ```bash
    # docker service ls
    ID            NAME                                     SCALE  IMAGE
    COMMAND
    29bv0vnlm903  vossibility-stack_lookupd                1 nsqio/nsq@sha256:eeba05599f31eba418e96e71e0984c3dc96963ceb66924dd37a47bf7ce18a662 /nsqlookupd
    4awt47624qwh  vossibility-stack_nsqd                   1 nsqio/nsq@sha256:eeba05599f31eba418e96e71e0984c3dc96963ceb66924dd37a47bf7ce18a662 /nsqd --data-path=/data --lookupd-tcp-address=lookupd:4160
    4tjx9biia6fs  vossibility-stack_elasticsearch          1 elasticsearch@sha256:12ac7c6af55d001f71800b83ba91a04f716e58d82e748fa6e5a7359eed2301aa
    7563uuzr9eys  vossibility-stack_kibana                 1 kibana@sha256:6995a2d25709a62694a937b8a529ff36da92ebee74bafd7bf00e6caf6db2eb03
    9gc5m4met4he  vossibility-stack_logstash               1 logstash@sha256:2dc8bddd1bb4a5a34e8ebaf73749f6413c101b2edef6617f2f7713926d2141fe logstash -f /etc/logstash/conf.d/logstash.conf
    axqh55ipl40h  vossibility-stack_vossibility-collector  1 icecrime/vossibility-collector@sha256:f03f2977203ba6253988c18d04061c5ec7aab46bca9dfd89a9a1fa4500989fba --config /config/config.toml --debug
    ```

## Managing stacks

Tasks are managed using the `docker stack` command:

    ```bash
    # docker stack --help

    Usage:  docker stack COMMAND
    
    Manage Docker stacks
    
    Options:
          --help   Print usage
    
    Commands:
      config      Print the stack configuration
      deploy      Create and update a stack
      rm          Remove the stack
      tasks       List the tasks in the stack
    
    Run 'docker stack COMMAND --help' for more information on a command.
    ```

---
title: "deploy"
description: "The deploy command description and usage"
keywords: "stack, deploy"
advisory: "experimental"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# deploy (alias for stack deploy) (experimental)

```markdown
Usage:  docker deploy [OPTIONS] STACK

Deploy a new stack or update an existing stack

Aliases:
  deploy, up

Options:
      --bundle-file string    Path to a Distributed Application Bundle file
      --compose-file string   Path to a Compose file
      --help                  Print usage
      --with-registry-auth    Send registry authentication details to Swarm agents
```

Create and update a stack from a `compose` or a `dab` file on the swarm. This command
has to be run targeting a manager node.

## Compose file

The `deploy` command supports compose file version `3.0` and above.

```bash
$ docker stack deploy --compose-file docker-compose.yml vossibility
Ignoring unsupported options: links

Creating network vossibility_vossibility
Creating network vossibility_default
Creating service vossibility_nsqd
Creating service vossibility_logstash
Creating service vossibility_elasticsearch
Creating service vossibility_kibana
Creating service vossibility_ghollector
Creating service vossibility_lookupd
```

You can verify that the services were correctly created

```
$ docker service ls
ID            NAME                               MODE        REPLICAS  IMAGE
29bv0vnlm903  vossibility_lookupd                replicated  1/1       nsqio/nsq@sha256:eeba05599f31eba418e96e71e0984c3dc96963ceb66924dd37a47bf7ce18a662
4awt47624qwh  vossibility_nsqd                   replicated  1/1       nsqio/nsq@sha256:eeba05599f31eba418e96e71e0984c3dc96963ceb66924dd37a47bf7ce18a662
4tjx9biia6fs  vossibility_elasticsearch          replicated  1/1       elasticsearch@sha256:12ac7c6af55d001f71800b83ba91a04f716e58d82e748fa6e5a7359eed2301aa
7563uuzr9eys  vossibility_kibana                 replicated  1/1       kibana@sha256:6995a2d25709a62694a937b8a529ff36da92ebee74bafd7bf00e6caf6db2eb03
9gc5m4met4he  vossibility_logstash               replicated  1/1       logstash@sha256:2dc8bddd1bb4a5a34e8ebaf73749f6413c101b2edef6617f2f7713926d2141fe
axqh55ipl40h  vossibility_vossibility-collector  replicated  1/1       icecrime/vossibility-collector@sha256:f03f2977203ba6253988c18d04061c5ec7aab46bca9dfd89a9a1fa4500989fba
```

## DAB file

```bash
$ docker stack deploy --bundle-file vossibility-stack.dab vossibility
Loading bundle from vossibility-stack.dab
Creating service vossibility_elasticsearch
Creating service vossibility_kibana
Creating service vossibility_logstash
Creating service vossibility_lookupd
Creating service vossibility_nsqd
Creating service vossibility_vossibility-collector
```

You can verify that the services were correctly created:

```bash
$ docker service ls
ID            NAME                               MODE        REPLICAS  IMAGE
29bv0vnlm903  vossibility_lookupd                replicated  1/1       nsqio/nsq@sha256:eeba05599f31eba418e96e71e0984c3dc96963ceb66924dd37a47bf7ce18a662
4awt47624qwh  vossibility_nsqd                   replicated  1/1       nsqio/nsq@sha256:eeba05599f31eba418e96e71e0984c3dc96963ceb66924dd37a47bf7ce18a662
4tjx9biia6fs  vossibility_elasticsearch          replicated  1/1       elasticsearch@sha256:12ac7c6af55d001f71800b83ba91a04f716e58d82e748fa6e5a7359eed2301aa
7563uuzr9eys  vossibility_kibana                 replicated  1/1       kibana@sha256:6995a2d25709a62694a937b8a529ff36da92ebee74bafd7bf00e6caf6db2eb03
9gc5m4met4he  vossibility_logstash               replicated  1/1       logstash@sha256:2dc8bddd1bb4a5a34e8ebaf73749f6413c101b2edef6617f2f7713926d2141fe
axqh55ipl40h  vossibility_vossibility-collector  replicated  1/1       icecrime/vossibility-collector@sha256:f03f2977203ba6253988c18d04061c5ec7aab46bca9dfd89a9a1fa4500989fba
```

## Related information

* [stack config](stack_config.md)
* [stack deploy](stack_deploy.md)
* [stack ls](stack_ls.md)
* [stack ps](stack_ps.md)
* [stack rm](stack_rm.md)
* [stack services](stack_services.md)

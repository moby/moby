---
title: "swarm unlock-key"
description: "The swarm unlock-keycommand description and usage"
keywords: "swarm, unlock-key"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# swarm unlock-key

```markdown
Usage:	docker swarm unlock-key [OPTIONS]

Manage the unlock key

Options:
      --help     Print usage
  -q, --quiet    Only display token
      --rotate   Rotate unlock key
```

## Description

An unlock key is a secret key needed to unlock a manager after its Docker daemon
restarts. These keys are only used when the autolock feature is enabled for the
swarm.

You can view or rotate the unlock key using `swarm unlock-key`. To view the key,
run the `docker swarm unlock-key` command without any arguments:

## Examples

```bash
$ docker swarm unlock-key

To unlock a swarm manager after it restarts, run the `docker swarm unlock`
command and provide the following key:

    SWMKEY-1-fySn8TY4w5lKcWcJPIpKufejh9hxx5KYwx6XZigx3Q4

Please remember to store this key in a password manager, since without it you
will not be able to restart the manager.
```

Use the `--rotate` flag to rotate the unlock key to a new, randomly-generated
key:

```bash
$ docker swarm unlock-key --rotate
Successfully rotated manager unlock key.

To unlock a swarm manager after it restarts, run the `docker swarm unlock`
command and provide the following key:

    SWMKEY-1-7c37Cc8654o6p38HnroywCi19pllOnGtbdZEgtKxZu8

Please remember to store this key in a password manager, since without it you
will not be able to restart the manager.
```

The `-q` (or `--quiet`) flag only prints the key:

```bash
$ docker swarm unlock-key -q
SWMKEY-1-7c37Cc8654o6p38HnroywCi19pllOnGtbdZEgtKxZu8
```

### `--rotate`

This flag rotates the unlock key, replacing it with a new randomly-generated
key. The old unlock key will no longer be accepted.

### `--quiet`

Only print the unlock key, without instructions.

## Related commands

* [swarm ca](swarm_ca.md)
* [swarm init](swarm_init.md)
* [swarm join](swarm_join.md)
* [swarm join-token](swarm_join_token.md)
* [swarm leave](swarm_leave.md)
* [swarm unlock](swarm_unlock.md)
* [swarm update](swarm_update.md)

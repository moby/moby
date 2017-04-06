---
title: "exec"
description: "The exec command description and usage"
keywords: "command, container, run, execute"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# exec

```markdown
Usage:  docker exec [OPTIONS] CONTAINER COMMAND [ARG...]

Run a command in a running container

Options:
  -d, --detach         Detached mode: run command in the background
      --detach-keys    Override the key sequence for detaching a container
  -e, --env=[]         Set environment variables
      --help           Print usage
  -i, --interactive    Keep STDIN open even if not attached
      --privileged     Give extended privileges to the command
  -t, --tty            Allocate a pseudo-TTY
  -u, --user           Username or UID (format: <name|uid>[:<group|gid>])
```

## Description

The `docker exec` command runs a new command in a running container.

The command started using `docker exec` only runs while the container's primary
process (`PID 1`) is running, and it is not restarted if the container is
restarted.

## Examples

### Run `docker exec` on a running container

First, start a container.

```bash
$ docker run --name ubuntu_bash --rm -i -t ubuntu bash
```

This will create a container named `ubuntu_bash` and start a Bash session.

Next, execute a command on the container.

```bash
$ docker exec -d ubuntu_bash touch /tmp/execWorks
```

This will create a new file `/tmp/execWorks` inside the running container
`ubuntu_bash`, in the background.

Next, execute an interactive `bash` shell on the container.

```bash
$ docker exec -it ubuntu_bash bash
```

This will create a new Bash session in the container `ubuntu_bash`.

### Try to run `docker exec` on a paused container

If the container is paused, then the `docker exec` command will fail with an error:

```bash
$ docker pause test

test

$ docker ps

CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS                   PORTS               NAMES
1ae3b36715d2        ubuntu:latest       "bash"              17 seconds ago      Up 16 seconds (Paused)                       test

$ docker exec test ls

FATA[0000] Error response from daemon: Container test is paused, unpause the container before exec

$ echo $?
1
```

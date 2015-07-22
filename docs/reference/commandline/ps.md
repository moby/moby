<!--[metadata]>
+++
title = "ps"
description = "The ps command description and usage"
keywords = ["container, running, list"]
[menu.main]
parent = "smn_cli"
weight=1
+++
<![end-metadata]-->

# ps

    Usage: docker ps [OPTIONS]

    List containers

      -a, --all=false       Show all containers (default shows just running)
      --before=""           Show only container created before Id or Name
      -f, --filter=[]       Filter output based on conditions provided
      -l, --latest=false    Show the latest created container, include non-running
      -n=-1                 Show n last created containers, include non-running
      --no-trunc=false      Don't truncate output
      -q, --quiet=false     Only display numeric IDs
      -s, --size=false      Display total file sizes
      --since=""            Show created since Id or Name, include non-running
      --format=[]       Pretty-print containers using a Go template

Running `docker ps --no-trunc` showing 2 linked containers.

    $ docker ps
    CONTAINER ID        IMAGE                        COMMAND                CREATED              STATUS              PORTS               NAMES
    4c01db0b339c        ubuntu:12.04                 bash                   17 seconds ago       Up 16 seconds       3300-3310/tcp       webapp
    d7886598dbe2        crosbymichael/redis:latest   /redis-server --dir    33 minutes ago       Up 33 minutes       6379/tcp            redis,webapp/db

`docker ps` will show only running containers by default. To see all containers:
`docker ps -a`

`docker ps` will group exposed ports into a single range if possible. E.g., a container that exposes TCP ports `100, 101, 102` will display `100-102/tcp` in the `PORTS` column.

## Filtering

The filtering flag (`-f` or `--filter)` format is a `key=value` pair. If there is more
than one filter, then pass multiple flags (e.g. `--filter "foo=bar" --filter "bif=baz"`)

The currently supported filters are:

* id (container's id)
* label (`label=<key>` or `label=<key>=<value>`)
* name (container's name)
* exited (int - the code of exited containers. Only useful with `--all`)
* status (created|restarting|running|paused|exited)

## Successfully exited containers

    $ docker ps -a --filter 'exited=0'
    CONTAINER ID        IMAGE             COMMAND                CREATED             STATUS                   PORTS                      NAMES
    ea09c3c82f6e        registry:latest   /srv/run.sh            2 weeks ago         Exited (0) 2 weeks ago   127.0.0.1:5000->5000/tcp   desperate_leakey
    106ea823fe4e        fedora:latest     /bin/sh -c 'bash -l'   2 weeks ago         Exited (0) 2 weeks ago                              determined_albattani
    48ee228c9464        fedora:20         bash                   2 weeks ago         Exited (0) 2 weeks ago                              tender_torvalds

This shows all the containers that have exited with status of '0'

## Formatting

The formatting option (`--format`) will pretty-print container output using a Go template.

Valid placeholders for the Go template are listed below:

Placeholder | Description
---- | ----
`.ID` | Container ID
`.Image` | Image ID
`.Command` | Quoted command
`.CreatedAt` | Time when the container was created.
`.RunningFor` | Elapsed time since the container was started.
`.Ports` | Exposed ports.
`.Status` | Container status.
`.Size` | Container disk size.
`.Labels` | All labels asigned to the container.
`.Label` | Value of a specific label for this container. For example `{{.Label "com.docker.swarm.cpu"}}`

When using the `--format` option, the `ps` command will either output the data exactly as the template
declares or, when using the `table` directive, will include column headers as well.

The following example uses a template without headers and outputs the `ID` and `Command`
entries separated by a colon for all running containers:

    $ docker ps --format "{{.ID}}: {{.Command}}"
    a87ecb4f327c: /bin/sh -c #(nop) MA
    01946d9d34d8: /bin/sh -c #(nop) MA
    c1d3b0166030: /bin/sh -c yum -y up
    41d50ecd2f57: /bin/sh -c #(nop) MA

To list all running containers with their labels in a table format you can use:

    $ docker ps --format "table {{.ID}}\t{{.Labels}}"
    CONTAINER ID        LABELS
    a87ecb4f327c        com.docker.swarm.node=ubuntu,com.docker.swarm.storage=ssd
    01946d9d34d8
    c1d3b0166030        com.docker.swarm.node=debian,com.docker.swarm.cpu=6
    41d50ecd2f57        com.docker.swarm.node=fedora,com.docker.swarm.cpu=3,com.docker.swarm.storage=ssd

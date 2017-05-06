List the containers in the local repository. By default this shows only
the running containers.

## Filters

Filter output based on these conditions:
   - exited=<int> an exit code of <int>
   - label=<key> or label=<key>=<value>
   - status=(created|restarting|running|paused|exited|dead)
   - name=<string> a container's name
   - id=<ID> a container's ID
   - is-task=(true|false) - containers that are a task (part of a service managed by swarm)
   - before=(<container-name>|<container-id>)
   - since=(<container-name>|<container-id>)
   - ancestor=(<image-name>[:tag]|<image-id>|<image@digest>) - containers created from an image or a descendant.
   - volume=(<volume-name>|<mount-point-destination>)
   - network=(<network-name>|<network-id>) - containers connected to the provided network
   - health=(starting|healthy|unhealthy|none) - filters containers based on healthcheck status
   - publish=(<port>[/<proto>]|<startport-endport>/[<proto>]) - filters containers based on published ports
   - expose=(<port>[/<proto>]|<startport-endport>/[<proto>]) - filters containers based on exposed ports

## Format

   Pretty-print containers using a Go template.
   Valid placeholders:
      .ID - Container ID
      .Image - Image ID
      .Command - Quoted command
      .CreatedAt - Time when the container was created.
      .RunningFor - Elapsed time since the container was started.
      .Ports - Exposed ports.
      .Status - Container status.
      .Size - Container disk size.
      .Names - Container names.
      .Labels - All labels assigned to the container.
      .Label - Value of a specific label for this container. For example `{{.Label "com.docker.swarm.cpu"}}`
      .Mounts - Names of the volumes mounted in this container.

# EXAMPLES
## Display all containers, including non-running

    $ docker container ls -a
    CONTAINER ID        IMAGE                 COMMAND                CREATED             STATUS      PORTS    NAMES
    a87ecb4f327c        fedora:20             /bin/sh -c #(nop) MA   20 minutes ago      Exit 0               desperate_brattain
    01946d9d34d8        vpavlin/rhel7:latest  /bin/sh -c #(nop) MA   33 minutes ago      Exit 0               thirsty_bell
    c1d3b0166030        acffc0358b9e          /bin/sh -c yum -y up   2 weeks ago         Exit 1               determined_torvalds
    41d50ecd2f57        fedora:20             /bin/sh -c #(nop) MA   2 weeks ago         Exit 0               drunk_pike

## Display only IDs of all containers, including non-running

    $ docker container ls -a -q
    a87ecb4f327c
    01946d9d34d8
    c1d3b0166030
    41d50ecd2f57

## Display only IDs of all containers that have the name `determined_torvalds`

    $ docker container ls -a -q --filter=name=determined_torvalds
    c1d3b0166030

## Display containers with their commands

    $ docker container ls --format "{{.ID}}: {{.Command}}"
    a87ecb4f327c: /bin/sh -c #(nop) MA
    01946d9d34d8: /bin/sh -c #(nop) MA
    c1d3b0166030: /bin/sh -c yum -y up
    41d50ecd2f57: /bin/sh -c #(nop) MA

## Display containers with their labels in a table

    $ docker container ls --format "table {{.ID}}\t{{.Labels}}"
    CONTAINER ID        LABELS
    a87ecb4f327c        com.docker.swarm.node=ubuntu,com.docker.swarm.storage=ssd
    01946d9d34d8
    c1d3b0166030        com.docker.swarm.node=debian,com.docker.swarm.cpu=6
    41d50ecd2f57        com.docker.swarm.node=fedora,com.docker.swarm.cpu=3,com.docker.swarm.storage=ssd

## Display containers with their node label in a table

    $ docker container ls --format 'table {{.ID}}\t{{(.Label "com.docker.swarm.node")}}'
    CONTAINER ID        NODE
    a87ecb4f327c        ubuntu
    01946d9d34d8
    c1d3b0166030        debian
    41d50ecd2f57        fedora

## Display containers with `remote-volume` mounted

    $ docker container ls --filter volume=remote-volume --format "table {{.ID}}\t{{.Mounts}}"
    CONTAINER ID        MOUNTS
    9c3527ed70ce        remote-volume

## Display containers with a volume mounted in `/data`

    $ docker container ls --filter volume=/data --format "table {{.ID}}\t{{.Mounts}}"
    CONTAINER ID        MOUNTS
    9c3527ed70ce        remote-volume

## Display containers that have published port of 80:

    $ docker ps --filter publish=80
    CONTAINER ID        IMAGE               COMMAND             CREATED              STATUS              PORTS                   NAMES
    fc7e477723b7        busybox             "top"               About a minute ago   Up About a minute   0.0.0.0:32768->80/tcp   admiring_roentgen

## Display containers that have exposed TCP port in the range of `8000-8080`:

    $ docker ps --filter expose=8000-8080/tcp
    CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS               NAMES
    9833437217a5        busybox             "top"               21 seconds ago      Up 19 seconds       8080/tcp            dreamy_mccarthy

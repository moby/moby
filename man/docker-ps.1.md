% DOCKER(1) Docker User Manuals
% Docker Community
% FEBRUARY 2015
# NAME
docker-ps - List containers

# SYNOPSIS
**docker ps**
[**-a**|**--all**]
[**-f**|**--filter**[=*[]*]]
[**--format**=*"TEMPLATE"*]
[**--help**]
[**-l**|**--latest**]
[**-n**[=*-1*]]
[**--no-trunc**]
[**-q**|**--quiet**]
[**-s**|**--size**]

# DESCRIPTION

List the containers in the local repository. By default this shows only
the running containers.

# OPTIONS
**-a**, **--all**=*true*|*false*
   Show all containers. Only running containers are shown by default. The default is *false*.

**-f**, **--filter**=[]
   Filter output based on these conditions:
   - exited=<int> an exit code of <int>
   - label=<key> or label=<key>=<value>
   - status=(created|restarting|running|paused|exited|dead)
   - name=<string> a container's name
   - id=<ID> a container's ID
   - before=(<container-name>|<container-id>)
   - since=(<container-name>|<container-id>)
   - ancestor=(<image-name>[:tag]|<image-id>|<image@digest>) - containers created from an image or a descendant.

**--format**="*TEMPLATE*"
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

**--help**
  Print usage statement

**-l**, **--latest**=*true*|*false*
   Show only the latest created container (includes all states). The default is *false*.

**-n**=*-1*
   Show n last created containers (includes all states).

**--no-trunc**=*true*|*false*
   Don't truncate output. The default is *false*.

**-q**, **--quiet**=*true*|*false*
   Only display numeric IDs. The default is *false*.

**-s**, **--size**=*true*|*false*
   Display total file sizes. The default is *false*.

# EXAMPLES
# Display all containers, including non-running

    # docker ps -a
    CONTAINER ID        IMAGE                 COMMAND                CREATED             STATUS      PORTS    NAMES
    a87ecb4f327c        fedora:20             /bin/sh -c #(nop) MA   20 minutes ago      Exit 0               desperate_brattain
    01946d9d34d8        vpavlin/rhel7:latest  /bin/sh -c #(nop) MA   33 minutes ago      Exit 0               thirsty_bell
    c1d3b0166030        acffc0358b9e          /bin/sh -c yum -y up   2 weeks ago         Exit 1               determined_torvalds
    41d50ecd2f57        fedora:20             /bin/sh -c #(nop) MA   2 weeks ago         Exit 0               drunk_pike

# Display only IDs of all containers, including non-running

    # docker ps -a -q
    a87ecb4f327c
    01946d9d34d8
    c1d3b0166030
    41d50ecd2f57

# Display only IDs of all containers that have the name `determined_torvalds`

    # docker ps -a -q --filter=name=determined_torvalds
    c1d3b0166030

# Display containers with their commands

    # docker ps --format "{{.ID}}: {{.Command}}"
    a87ecb4f327c: /bin/sh -c #(nop) MA
    01946d9d34d8: /bin/sh -c #(nop) MA
    c1d3b0166030: /bin/sh -c yum -y up
    41d50ecd2f57: /bin/sh -c #(nop) MA

# Display containers with their labels in a table

    # docker ps --format "table {{.ID}}\t{{.Labels}}"
    CONTAINER ID        LABELS
    a87ecb4f327c        com.docker.swarm.node=ubuntu,com.docker.swarm.storage=ssd
    01946d9d34d8
    c1d3b0166030        com.docker.swarm.node=debian,com.docker.swarm.cpu=6
    41d50ecd2f57        com.docker.swarm.node=fedora,com.docker.swarm.cpu=3,com.docker.swarm.storage=ssd

# Display containers with their node label in a table

    # docker ps --format 'table {{.ID}}\t{{(.Label "com.docker.swarm.node")}}'
    CONTAINER ID        NODE
    a87ecb4f327c        ubuntu
    01946d9d34d8
    c1d3b0166030        debian
    41d50ecd2f57        fedora

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
August 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
November 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
February 2015, updated by André Martins <martins@noironetworks.com>

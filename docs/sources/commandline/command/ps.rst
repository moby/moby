:title: Ps Command
:description: List containers
:keywords: ps, docker, documentation, container

=========================
``ps`` -- List containers
=========================

::

   Usage: docker ps [-ahlqs] [--before id] [-n value] [--no-trunc] [--since id]

   List containers

    -a, --all        Show all containers. Only running containers are shown by
 		     default.
        --before=id  Show only container created before Id, include non-running
                     ones.
    -h, --help       Display this help
    -l, --latest     Show only the latest created container, include non-running
                     ones.
    -n value         Show n last created containers, include non-running ones.
        --no-trunc   Don't truncate output
    -q, --quiet      Only display numeric IDs
        --since=id   Show only containers created since Id, include non-running
                     ones.
    -s, --size       Display sizes
 

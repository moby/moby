<!--[metadata]>
+++
title = "volume ls"
description = "The volume ls command description and usage"
keywords = ["volume, list"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# volume ls

    Usage: docker volume ls [OPTIONS]

    List volumes

      -f, --filter=[]      Provide filter values (i.e. 'dangling=true')
      --help=false         Print usage
      -q, --quiet=false    Only display volume names

Lists all the volumes Docker knows about. You can filter using the `-f` or `--filter` flag. The filtering format is a `key=value` pair. To specify more than one filter,  pass multiple flags (for example,  `--filter "foo=bar" --filter "bif=baz"`)

There is a single supported filter `dangling=value` which takes a boolean of `true` or `false`.

Example output:

    $ docker volume create --name rose
    rose
    $docker volume create --name tyler
    tyler
    $ docker volume ls
    DRIVER              VOLUME NAME
    local               rose
    local               tyler

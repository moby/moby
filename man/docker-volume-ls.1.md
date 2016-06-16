% DOCKER(1) Docker User Manuals
% Docker Community
% JULY 2015
# NAME
docker-volume-ls - List all volumes

# SYNOPSIS
**docker volume ls**
[**-f**|**--filter**[=*FILTER*]]
[**--help**]
[**-q**|**--quiet**[=*true*|*false*]]

# DESCRIPTION

Lists all the volumes Docker knows about. You can filter using the `-f` or `--filter` flag. The filtering format is a `key=value` pair. To specify more than one filter,  pass multiple flags (for example,  `--filter "foo=bar" --filter "bif=baz"`)

There is a single supported filter `dangling=value` which takes a boolean of `true` or `false`.

# OPTIONS
**-f**, **--filter**=""
  Filter output based on these conditions:
  - dangling=<boolean> a volume if referenced or not
  - driver=<string> a volume's driver name
  - name=<string> a volume's name

**--help**
  Print usage statement

**-q**, **--quiet**=*true*|*false*
  Only display volume names

# HISTORY
July 2015, created by Brian Goff <cpuguy83@gmail.com>

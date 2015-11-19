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
  Provide filter values (i.e. 'dangling=true')

**--help**
  Print usage statement

**-q**, **--quiet**=*true*|*false*
  Only display volume names

# HISTORY
July 2015, created by Brian Goff <cpuguy83@gmail.com>

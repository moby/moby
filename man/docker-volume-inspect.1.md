% DOCKER(1) Docker User Manuals
% Docker Community
% JULY 2015
# NAME
docker-volume-inspect - Get low-level information about a volume

# SYNOPSIS
**docker volume inspect**
[**-f**|**--format**[=*FORMAT*]]
[**--help**]
VOLUME [VOLUME...]

# DESCRIPTION

Returns information about one or more volumes. By default, this command renders all results
in a JSON array. You can specify an alternate format to execute a given template
is executed for each result. Go's
http://golang.org/pkg/text/template/ package describes all the details of the
format.

# OPTIONS
**-f**, **--format**=""
  Format the output using the given go template.

**--help**
  Print usage statement

# HISTORY
July 2015, created by Brian Goff <cpuguy83@gmail.com>

---
title: "search"
description: "The search command description and usage"
keywords: "search, hub, images"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# search

```markdown
Usage:  docker search [OPTIONS] TERM

Search the Docker Hub for images

Options:
  -f, --filter value   Filter output based on conditions provided (default [])
                       - is-automated=(true|false)
                       - is-official=(true|false)
                       - stars=<number> - image has at least 'number' stars
      --help           Print usage
      --limit int      Max number of search results (default 25)
      --no-trunc       Don't truncate output
```

## Description

Search [Docker Hub](https://hub.docker.com) for images

See [*Find Public Images on Docker Hub*](https://docs.docker.com/engine/tutorials/dockerrepos/#searching-for-images) for
more details on finding shared images from the command line.

> **Note**: Search queries return a maximum of 25 results.

## Examples

### Search images by name

This example displays images with a name containing 'busybox':

```none
$ docker search busybox

NAME                             DESCRIPTION                                     STARS     OFFICIAL   AUTOMATED
busybox                          Busybox base image.                             316       [OK]       
progrium/busybox                                                                 50                   [OK]
radial/busyboxplus               Full-chain, Internet enabled, busybox made...   8                    [OK]
odise/busybox-python                                                             2                    [OK]
azukiapp/busybox                 This image is meant to be used as the base...   2                    [OK]
ofayau/busybox-jvm               Prepare busybox to install a 32 bits JVM.       1                    [OK]
shingonoide/archlinux-busybox    Arch Linux, a lightweight and flexible Lin...   1                    [OK]
odise/busybox-curl                                                               1                    [OK]
ofayau/busybox-libc32            Busybox with 32 bits (and 64 bits) libs         1                    [OK]
peelsky/zulu-openjdk-busybox                                                     1                    [OK]
skomma/busybox-data              Docker image suitable for data volume cont...   1                    [OK]
elektritter/busybox-teamspeak    Lightweight teamspeak3 container based on...    1                    [OK]
socketplane/busybox                                                              1                    [OK]
oveits/docker-nginx-busybox      This is a tiny NginX docker image based on...   0                    [OK]
ggtools/busybox-ubuntu           Busybox ubuntu version with extra goodies       0                    [OK]
nikfoundas/busybox-confd         Minimal busybox based distribution of confd     0                    [OK]
openshift/busybox-http-app                                                       0                    [OK]
jllopis/busybox                                                                  0                    [OK]
swyckoff/busybox                                                                 0                    [OK]
powellquiring/busybox                                                            0                    [OK]
williamyeh/busybox-sh            Docker image for BusyBox's sh                   0                    [OK]
simplexsys/busybox-cli-powered   Docker busybox images, with a few often us...   0                    [OK]
fhisamoto/busybox-java           Busybox java                                    0                    [OK]
scottabernethy/busybox                                                           0                    [OK]
marclop/busybox-solr
```

### Display non-truncated description (--no-trunc)

This example displays images with a name containing 'busybox',
at least 3 stars and the description isn't truncated in the output:

```bash
$ docker search --stars=3 --no-trunc busybox
NAME                 DESCRIPTION                                                                               STARS     OFFICIAL   AUTOMATED
busybox              Busybox base image.                                                                       325       [OK]       
progrium/busybox                                                                                               50                   [OK]
radial/busyboxplus   Full-chain, Internet enabled, busybox made from scratch. Comes in git and cURL flavors.   8                    [OK]
```

### Limit search results (--limit)

The flag `--limit` is the maximum number of results returned by a search. This value could
be in the range between 1 and 100. The default value of `--limit` is 25.


### Filtering

The filtering flag (`-f` or `--filter`) format is a `key=value` pair. If there is more
than one filter, then pass multiple flags (e.g. `--filter "foo=bar" --filter "bif=baz"`)

The currently supported filters are:

* stars (int - number of stars the image has)
* is-automated (true|false) - is the image automated or not
* is-official (true|false) - is the image official or not


#### stars

This example displays images with a name containing 'busybox' and at
least 3 stars:

```bash
$ docker search --filter stars=3 busybox

NAME                 DESCRIPTION                                     STARS     OFFICIAL   AUTOMATED
busybox              Busybox base image.                             325       [OK]       
progrium/busybox                                                     50                   [OK]
radial/busyboxplus   Full-chain, Internet enabled, busybox made...   8                    [OK]
```


#### is-automated

This example displays images with a name containing 'busybox'
and are automated builds:

```bash
$ docker search --filter is-automated busybox

NAME                 DESCRIPTION                                     STARS     OFFICIAL   AUTOMATED
progrium/busybox                                                     50                   [OK]
radial/busyboxplus   Full-chain, Internet enabled, busybox made...   8                    [OK]
```

#### is-official

This example displays images with a name containing 'busybox', at least
3 stars and are official builds:

```bash
$ docker search --filter "is-official=true" --filter "stars=3" busybox

NAME                 DESCRIPTION                                     STARS     OFFICIAL   AUTOMATED
progrium/busybox                                                     50                   [OK]
radial/busyboxplus   Full-chain, Internet enabled, busybox made...   8                    [OK]
```

<!--[metadata]>
+++
title = "search"
description = "The search command description and usage"
keywords = ["search, hub, images"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# search

    Usage: docker search [OPTIONS] TERM

    Search the Docker Hub for images

      --automated=false    Only show automated builds
      --help=false         Print usage
      --no-trunc=false     Don't truncate output
      -s, --stars=0        Only displays with at least x stars

Search [Docker Hub](https://hub.docker.com) for images

See [*Find Public Images on Docker Hub*](../../userguide/dockerrepos.md#searching-for-images) for
more details on finding shared images from the command line.

> **Note:**
> Search queries will only return up to 25 results

## Examples

### Search images by name

This example displays images with a name containing 'busybox':

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
    elektritter/busybox-teamspeak    Leightweight teamspeak3 container based on...   1                    [OK]
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

### Search images by name and number of stars (-s, --stars)

This example displays images with a name containing 'busybox' and at
least 3 stars:

    $ docker search --stars=3 busybox
    NAME                 DESCRIPTION                                     STARS     OFFICIAL   AUTOMATED
    busybox              Busybox base image.                             325       [OK]       
    progrium/busybox                                                     50                   [OK]
    radial/busyboxplus   Full-chain, Internet enabled, busybox made...   8                    [OK]


### Search automated images (--automated)

This example displays images with a name containing 'busybox', at
least 3 stars and are automated builds:

    $ docker search --stars=3 --automated busybox
    NAME                 DESCRIPTION                                     STARS     OFFICIAL   AUTOMATED
    progrium/busybox                                                     50                   [OK]
    radial/busyboxplus   Full-chain, Internet enabled, busybox made...   8                    [OK]


### Display non-truncated description (--no-trunc)

This example displays images with a name containing 'busybox',
at least 3 stars and the description isn't truncated in the output:

    $ docker search --stars=3 --no-trunc busybox
    NAME                 DESCRIPTION                                                                               STARS     OFFICIAL   AUTOMATED
    busybox              Busybox base image.                                                                       325       [OK]       
    progrium/busybox                                                                                               50                   [OK]
    radial/busyboxplus   Full-chain, Internet enabled, busybox made from scratch. Comes in git and cURL flavors.   8                    [OK]


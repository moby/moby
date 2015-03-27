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

      --automated          Only show automated builds
      --help               Print usage
      --no-index           Omit index column from output
      --no-trunc           Don't truncate output
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
    INDEX       NAME                                       DESCRIPTION                                     STARS     OFFICIAL   AUTOMATED
    docker.io   docker.io/busybox                          Busybox base image.                             436       [OK]
    docker.io   docker.io/progrium/busybox                                                                 53                   [OK]
    docker.io   docker.io/radial/busyboxplus               Full-chain, Internet enabled, busybox made...   8                    [OK]
    docker.io   docker.io/odise/busybox-python                                                             3                    [OK]
    docker.io   docker.io/azukiapp/busybox                 This image is meant to be used as the base...   2                    [OK]
    docker.io   docker.io/multiarch/busybox                multiarch ports of ubuntu-debootstrap           2                    [OK]
    docker.io   docker.io/elektritter/busybox-teamspeak    Leightweight teamspeak3 container based on...   1                    [OK]
    docker.io   docker.io/odise/busybox-curl                                                               1                    [OK]
    docker.io   docker.io/ofayau/busybox-jvm               Prepare busybox to install a 32 bits JVM.       1                    [OK]
    docker.io   docker.io/ofayau/busybox-libc32            Busybox with 32 bits (and 64 bits) libs         1                    [OK]
    docker.io   docker.io/peelsky/zulu-openjdk-busybox                                                     1                    [OK]
    docker.io   docker.io/sequenceiq/busybox                                                               1                    [OK]
    docker.io   docker.io/shingonoide/archlinux-busybox    Arch Linux, a lightweight and flexible Lin...   1                    [OK]
    docker.io   docker.io/skomma/busybox-data              Docker image suitable for data volume cont...   1                    [OK]
    docker.io   docker.io/socketplane/busybox                                                              1                    [OK]
    docker.io   docker.io/buddho/busybox-java8             Java8 on Busybox                                0                    [OK]
    docker.io   docker.io/container4armhf/armhf-busybox    Automated build of Busybox for armhf devic...   0                    [OK]
    docker.io   docker.io/ggtools/busybox-ubuntu           Busybox ubuntu version with extra goodies       0                    [OK]
    docker.io   docker.io/nikfoundas/busybox-confd         Minimal busybox based distribution of confd     0                    [OK]
    docker.io   docker.io/openshift/busybox-http-app                                                       0                    [OK]
    docker.io   docker.io/oveits/docker-nginx-busybox      This is a tiny NginX docker image based on...   0                    [OK]
    docker.io   docker.io/powellquiring/busybox                                                            0                    [OK]
    docker.io   docker.io/simplexsys/busybox-cli-powered   Docker busybox images, with a few often us...   0                    [OK]
    docker.io   docker.io/stolus/busybox                                                                   0                    [OK]
    docker.io   docker.io/williamyeh/busybox-sh            Docker image for BusyBox's sh                   0                    [OK]

### Search images by name and number of stars (-s, --stars)

This example displays images with a name containing 'busybox' and at
least 3 stars:

    $ docker search --stars=3 busybox
    INDEX       NAME                             DESCRIPTION                                     STARS     OFFICIAL   AUTOMATED
    docker.io   docker.io/busybox                Busybox base image.                             436       [OK]
    docker.io   docker.io/progrium/busybox                                                       53                   [OK]
    docker.io   docker.io/radial/busyboxplus     Full-chain, Internet enabled, busybox made...   8                    [OK]
    docker.io   docker.io/odise/busybox-python                                                   3                    [OK]

### Search automated images (--automated)

This example displays images with a name containing 'busybox', at
least 3 stars and are automated builds:

    $ docker search --stars=3 --automated busybox
    INDEX       NAME                             DESCRIPTION                                     STARS     OFFICIAL   AUTOMATED
    docker.io   docker.io/progrium/busybox                                                       53                   [OK]
    docker.io   docker.io/radial/busyboxplus     Full-chain, Internet enabled, busybox made...   8                    [OK]
    docker.io   docker.io/odise/busybox-python                                                   3                    [OK]

### Display non-truncated description (--no-trunc)

This example displays images with a name containing 'busybox',
at least 3 stars and the description isn't truncated in the output:

    $ docker search --stars=3 --no-trunc busybox
    INDEX       NAME                             DESCRIPTION                                                                               STARS     OFFICIAL   AUTOMATED
    docker.io   docker.io/busybox                Busybox base image.                                                                       436       [OK]
    docker.io   docker.io/progrium/busybox                                                                                                 53                   [OK]
    docker.io   docker.io/radial/busyboxplus     Full-chain, Internet enabled, busybox made from scratch. Comes in git and cURL flavors.   8                    [OK]
    docker.io   docker.io/odise/busybox-python                                                                                             3                    [OK]

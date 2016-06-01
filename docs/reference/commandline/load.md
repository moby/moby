<!--[metadata]>
+++
title = "load"
description = "The load command description and usage"
keywords = ["stdin, tarred, repository"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# load

    Usage: docker load [OPTIONS]

    Load an image from a tar archive or STDIN

      --help             Print usage
      -i, --input=""     Read from a tar archive file, instead of STDIN. The tarball may be compressed with gzip, bzip, or xz
      -q, --quiet        Suppress the load output. Without this option, a progress bar is displayed.

Loads a tarred repository from a file or the standard input stream.
Restores both images and tags.

    $ docker images
    REPOSITORY          TAG                 IMAGE ID            CREATED             SIZE
    $ docker load < busybox.tar.gz
    $ docker images
    REPOSITORY          TAG                 IMAGE ID            CREATED             SIZE
    busybox             latest              769b9341d937        7 weeks ago         2.489 MB
    $ docker load --input fedora.tar
    $ docker images
    REPOSITORY          TAG                 IMAGE ID            CREATED             SIZE
    busybox             latest              769b9341d937        7 weeks ago         2.489 MB
    fedora              rawhide             0d20aec6529d        7 weeks ago         387 MB
    fedora              20                  58394af37342        7 weeks ago         385.5 MB
    fedora              heisenbug           58394af37342        7 weeks ago         385.5 MB
    fedora              latest              58394af37342        7 weeks ago         385.5 MB

Loads a tarred repository from a file or the standard input stream.
Restores both images and tags. Write image names or IDs imported it
standard output stream.

# EXAMPLES

    $ docker images
    REPOSITORY          TAG                 IMAGE ID            CREATED             SIZE
    busybox             latest              769b9341d937        7 weeks ago         2.489 MB
    $ docker load --input fedora.tar
    # […]
    Loaded image: fedora:rawhide
    # […]
    Loaded image: fedora:20
    # […]
    $ docker images
    REPOSITORY          TAG                 IMAGE ID            CREATED             SIZE
    busybox             latest              769b9341d937        7 weeks ago         2.489 MB
    fedora              rawhide             0d20aec6529d        7 weeks ago         387 MB
    fedora              20                  58394af37342        7 weeks ago         385.5 MB
    fedora              heisenbug           58394af37342        7 weeks ago         385.5 MB
    fedora              latest              58394af37342        7 weeks ago         385.5 MB

# See also
**docker-image-save(1)** to save one or more images to a tar archive (streamed to STDOUT by default).

Produces a tarred repository to the standard output stream. Contains all
parent layers, and all tags + versions, or specified repo:tag.

Stream to a file instead of STDOUT by using **-o**.

# EXAMPLES

Save all fedora repository images to a fedora-all.tar and save the latest
fedora image to a fedora-latest.tar:

    $ docker image save fedora > fedora-all.tar
    $ docker image save --output=fedora-latest.tar fedora:latest
    $ ls -sh fedora-all.tar
    721M fedora-all.tar
    $ ls -sh fedora-latest.tar
    367M fedora-latest.tar

# See also
**docker-image-load(1)** to load an image from a tar archive on STDIN.

page_title: Create a Base Image
page_description: How to create base images
page_keywords: Examples, Usage, base image, docker, documentation, examples

# Create a Base Image

So you want to create your own [*Base Image*](
/terms/image/#base-image-def)? Great!

The specific process will depend heavily on the Linux distribution you
want to package. We have some examples below, and you are encouraged to
submit pull requests to contribute new ones.

## Create a full image using tar

In general, you'll want to start with a working machine that is running
the distribution you'd like to package as a base image, though that is
not required for some tools like Debian's
[Debootstrap](https://wiki.debian.org/Debootstrap), which you can also
use to build Ubuntu images.

It can be as simple as this to create an Ubuntu base image:

    $ sudo debootstrap raring raring > /dev/null
    $ sudo tar -C raring -c . | sudo docker import - raring
    a29c15f1bf7a
    $ sudo docker run raring cat /etc/lsb-release
    DISTRIB_ID=Ubuntu
    DISTRIB_RELEASE=13.04
    DISTRIB_CODENAME=raring
    DISTRIB_DESCRIPTION="Ubuntu 13.04"

There are more example scripts for creating base images in the Docker
GitHub Repo:

 - [BusyBox](https://github.com/docker/docker/blob/master/contrib/mkimage-busybox.sh)
 - CentOS / Scientific Linux CERN (SLC) [on Debian/Ubuntu](
   https://github.com/docker/docker/blob/master/contrib/mkimage-rinse.sh) or
   [on CentOS/RHEL/SLC/etc.](
   https://github.com/docker/docker/blob/master/contrib/mkimage-yum.sh)
 - [Debian / Ubuntu](
   https://github.com/docker/docker/blob/master/contrib/mkimage-debootstrap.sh)

## Creating a simple base image using `scratch`

There is a special repository in the Docker registry called `scratch`, which
was created using an empty tar file:

    $ tar cv --files-from /dev/null | docker import - scratch

which you can `docker pull`. You can then use that
image to base your new minimal containers `FROM`:

    FROM scratch
    COPY true-asm /true
    CMD ["/true"]

The `Dockerfile` above is from an extremely minimal image - [tianon/true](
https://github.com/tianon/dockerfiles/tree/master/true).

## More resources

There are lots more resources available to help you write your 'Dockerfile`.

* There's a [complete guide to all the instructions](/reference/builder/) available for use in a `Dockerfile` in the reference section.
* To help you write a clear, readable, maintainable `Dockerfile`, we've also
written a [`Dockerfile` Best Practices guide](/articles/dockerfile_best-practices).
* If you're working on an Official Repo, be sure to check out the [Official Repo Guidelines](/docker-hub/official_repos/).

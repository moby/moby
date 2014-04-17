page_title: Running an apt-cacher-ng service
page_description: Installing and running an apt-cacher-ng service
page_keywords: docker, example, package installation, networking, debian, ubuntu

# Apt-Cacher-ng Service

Note

-   This example assumes you have Docker running in daemon mode. For
    more information please see [*Check your Docker
    install*](../hello_world/#running-examples).
-   **If you don’t like sudo** then see [*Giving non-root
    access*](../../installation/binaries/#dockergroup)
-   **If you’re using OS X or docker via TCP** then you shouldn’t use
    sudo

When you have multiple Docker servers, or build unrelated Docker
containers which can’t make use of the Docker build cache, it can be
useful to have a caching proxy for your packages. This container makes
the second download of any package almost instant.

Use the following Dockerfile:

    #
    # Build: docker build -t apt-cacher .
    # Run: docker run -d -p 3142:3142 --name apt-cacher-run apt-cacher
    #
    # and then you can run containers with:
    #   docker run -t -i --rm -e http_proxy http://dockerhost:3142/ debian bash
    #
    FROM        ubuntu
    MAINTAINER  SvenDowideit@docker.com

    VOLUME      ["/var/cache/apt-cacher-ng"]
    RUN     apt-get update ; apt-get install -yq apt-cacher-ng

    EXPOSE      3142
    CMD     chmod 777 /var/cache/apt-cacher-ng ; /etc/init.d/apt-cacher-ng start ; tail -f /var/log/apt-cacher-ng/*

To build the image using:

    $ sudo docker build -t eg_apt_cacher_ng .

Then run it, mapping the exposed port to one on the host

    $ sudo docker run -d -p 3142:3142 --name test_apt_cacher_ng eg_apt_cacher_ng

To see the logfiles that are ‘tailed’ in the default command, you can
use:

    $ sudo docker logs -f test_apt_cacher_ng

To get your Debian-based containers to use the proxy, you can do one of
three things

1.  Add an apt Proxy setting
    `echo 'Acquire::http { Proxy "http://dockerhost:3142"; };' >> /etc/apt/conf.d/01proxy`

2.  Set an environment variable:
    `http_proxy=http://dockerhost:3142/`
3.  Change your `sources.list` entries to start with
    `http://dockerhost:3142/`

**Option 1** injects the settings safely into your apt configuration in
a local version of a common base:

    FROM ubuntu
    RUN  echo 'Acquire::http { Proxy "http://dockerhost:3142"; };' >> /etc/apt/apt.conf.d/01proxy
    RUN apt-get update ; apt-get install vim git

    # docker build -t my_ubuntu .

**Option 2** is good for testing, but will break other HTTP clients
which obey `http_proxy`, such as `curl`
.literal}, `wget` and others:

    $ sudo docker run --rm -t -i -e http_proxy=http://dockerhost:3142/ debian bash

**Option 3** is the least portable, but there will be times when you
might need to do it and you can do it from your `Dockerfile`
too.

Apt-cacher-ng has some tools that allow you to manage the repository,
and they can be used by leveraging the `VOLUME`
instruction, and the image we built to run the service:

    $ sudo docker run --rm -t -i --volumes-from test_apt_cacher_ng eg_apt_cacher_ng bash

    $$ /usr/lib/apt-cacher-ng/distkill.pl
    Scanning /var/cache/apt-cacher-ng, please wait...
    Found distributions:
    bla, taggedcount: 0
         1. precise-security (36 index files)
         2. wheezy (25 index files)
         3. precise-updates (36 index files)
         4. precise (36 index files)
         5. wheezy-updates (18 index files)

    Found architectures:
         6. amd64 (36 index files)
         7. i386 (24 index files)

    WARNING: The removal action may wipe out whole directories containing
             index files. Select d to see detailed list.

    (Number nn: tag distribution or architecture nn; 0: exit; d: show details; r: remove tagged; q: quit): q

Finally, clean up after your test by stopping and removing the
container, and then removing the image.

    $ sudo docker stop test_apt_cacher_ng
    $ sudo docker rm test_apt_cacher_ng
    $ sudo docker rmi eg_apt_cacher_ng

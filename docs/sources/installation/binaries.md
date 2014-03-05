Binaries[¶](#binaries "Permalink to this headline")
===================================================

Note

Docker is still under heavy development! We don’t recommend using it in
production yet, but we’re getting closer with each release. Please see
our blog post, [“Getting to Docker
1.0”](http://blog.docker.io/2013/08/getting-to-docker-1-0/)

**This instruction set is meant for hackers who want to try out Docker
on a variety of environments.**

Before following these directions, you should really check if a packaged
version of Docker is already available for your distribution. We have
packages for many distributions, and more keep showing up all the time!

Check runtime dependencies[¶](#check-runtime-dependencies "Permalink to this headline")
---------------------------------------------------------------------------------------

To run properly, docker needs the following software to be installed at
runtime:

-   iproute2 version 3.5 or later (build after 2012-05-21), and
    specifically the “ip” utility
-   iptables version 1.4 or later
-   The LXC utility scripts
    ([http://lxc.sourceforge.net](http://lxc.sourceforge.net)) version
    0.8 or later
-   Git version 1.7 or later
-   XZ Utils 4.9 or later

Check kernel dependencies[¶](#check-kernel-dependencies "Permalink to this headline")
-------------------------------------------------------------------------------------

Docker in daemon mode has specific kernel requirements. For details,
check your distribution in [*Installation*](../#installation-list).

Note that Docker also has a client mode, which can run on virtually any
linux kernel (it even builds on OSX!).

Get the docker binary:[¶](#get-the-docker-binary "Permalink to this headline")
------------------------------------------------------------------------------

    wget https://get.docker.io/builds/Linux/x86_64/docker-latest -O docker
    chmod +x docker

Run the docker daemon[¶](#run-the-docker-daemon "Permalink to this headline")
-----------------------------------------------------------------------------

    # start the docker in daemon mode from the directory you unpacked
    sudo ./docker -d &

Giving non-root access[¶](#giving-non-root-access "Permalink to this headline")
-------------------------------------------------------------------------------

The `docker`{.docutils .literal} daemon always runs as the root user,
and since Docker version 0.5.2, the `docker`{.docutils .literal} daemon
binds to a Unix socket instead of a TCP port. By default that Unix
socket is owned by the user *root*, and so, by default, you can access
it with `sudo`{.docutils .literal}.

Starting in version 0.5.3, if you (or your Docker installer) create a
Unix group called *docker* and add users to it, then the
`docker`{.docutils .literal} daemon will make the ownership of the Unix
socket read/writable by the *docker* group when the daemon starts. The
`docker`{.docutils .literal} daemon must always run as the root user,
but if you run the `docker`{.docutils .literal} client as a user in the
*docker* group then you don’t need to add `sudo`{.docutils .literal} to
all the client commands.

Warning

The *docker* group is root-equivalent.

Upgrades[¶](#upgrades "Permalink to this headline")
---------------------------------------------------

To upgrade your manual installation of Docker, first kill the docker
daemon:

    killall docker

Then follow the regular installation steps.

Run your first container![¶](#run-your-first-container "Permalink to this headline")
------------------------------------------------------------------------------------

    # check your docker version
    sudo ./docker version

    # run a container and open an interactive shell in the container
    sudo ./docker run -i -t ubuntu /bin/bash

Continue with the [*Hello
World*](../../examples/hello_world/#hello-world) example.

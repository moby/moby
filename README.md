Docker: the Linux container engine
==================================

Docker is an open source project to pack, ship and run any application
as a lightweight container

Docker containers are both *hardware-agnostic* and *platform-agnostic*.
This means that they can run anywhere, from your laptop to the largest
EC2 compute instance and everything in between - and they don't require
that you use a particular language, framework or packaging system. That
makes them great building blocks for deploying and scaling web apps,
databases and backend services without depending on a particular stack
or provider.

Docker is an open-source implementation of the deployment engine which
powers [dotCloud](http://dotcloud.com), a popular Platform-as-a-Service.
It benefits directly from the experience accumulated over several years
of large-scale operation and support of hundreds of thousands of
applications and databases.

![Docker L](docs/theme/mkdocs/images/docker-logo-compressed.png "Docker")

## Security Disclosure

Security is very important to us.  If you have any issue regarding security, 
please disclose the information responsibly by sending an email to 
security@docker.com and not by creating a github issue.

## Better than VMs

A common method for distributing applications and sandboxing their
execution is to use virtual machines, or VMs. Typical VM formats are
VMWare's vmdk, Oracle Virtualbox's vdi, and Amazon EC2's ami. In theory
these formats should allow every developer to automatically package
their application into a "machine" for easy distribution and deployment.
In practice, that almost never happens, for a few reasons:

  * *Size*: VMs are very large which makes them impractical to store
     and transfer.
  * *Performance*: running VMs consumes significant CPU and memory,
    which makes them impractical in many scenarios, for example local
    development of multi-tier applications, and large-scale deployment
    of cpu and memory-intensive applications on large numbers of
    machines.
  * *Portability*: competing VM environments don't play well with each
     other. Although conversion tools do exist, they are limited and
     add even more overhead.
  * *Hardware-centric*: VMs were designed with machine operators in
    mind, not software developers. As a result, they offer very
    limited tooling for what developers need most: building, testing
    and running their software. For example, VMs offer no facilities
    for application versioning, monitoring, configuration, logging or
    service discovery.

By contrast, Docker relies on a different sandboxing method known as
*containerization*. Unlike traditional virtualization, containerization
takes place at the kernel level. Most modern operating system kernels
now support the primitives necessary for containerization, including
Linux with [openvz](http://openvz.org),
[vserver](http://linux-vserver.org) and more recently
[lxc](http://lxc.sourceforge.net), Solaris with
[zones](http://docs.oracle.com/cd/E26502_01/html/E29024/preface-1.html#scrolltoc)
and FreeBSD with
[Jails](http://www.freebsd.org/doc/handbook/jails.html).

Docker builds on top of these low-level primitives to offer developers a
portable format and runtime environment that solves all 4 problems.
Docker containers are small (and their transfer can be optimized with
layers), they have basically zero memory and cpu overhead, they are
completely portable and are designed from the ground up with an
application-centric design.

The best part: because Docker operates at the OS level, it can still be
run inside a VM!

## Plays well with others

Docker does not require that you buy into a particular programming
language, framework, packaging system or configuration language.

Is your application a Unix process? Does it use files, tcp connections,
environment variables, standard Unix streams and command-line arguments
as inputs and outputs? Then Docker can run it.

Can your application's build be expressed as a sequence of such
commands? Then Docker can build it.

## Escape dependency hell

A common problem for developers is the difficulty of managing all
their application's dependencies in a simple and automated way.

This is usually difficult for several reasons:

  * *Cross-platform dependencies*. Modern applications often depend on
    a combination of system libraries and binaries, language-specific
    packages, framework-specific modules, internal components
    developed for another project, etc. These dependencies live in
    different "worlds" and require different tools - these tools
    typically don't work well with each other, requiring awkward
    custom integrations.

  * Conflicting dependencies. Different applications may depend on
    different versions of the same dependency. Packaging tools handle
    these situations with various degrees of ease - but they all
    handle them in different and incompatible ways, which again forces
    the developer to do extra work.
  
  * Custom dependencies. A developer may need to prepare a custom
    version of their application's dependency. Some packaging systems
    can handle custom versions of a dependency, others can't - and all
    of them handle it differently.


Docker solves dependency hell by giving the developer a simple way to
express *all* their application's dependencies in one place, and
streamline the process of assembling them. If this makes you think of
[XKCD 927](http://xkcd.com/927/), don't worry. Docker doesn't
*replace* your favorite packaging systems. It simply orchestrates
their use in a simple and repeatable way. How does it do that? With
layers.

Docker defines a build as running a sequence of Unix commands, one
after the other, in the same container. Build commands modify the
contents of the container (usually by installing new files on the
filesystem), the next command modifies it some more, etc. Since each
build command inherits the result of the previous commands, the
*order* in which the commands are executed expresses *dependencies*.

Here's a typical Docker build process:

```bash
FROM ubuntu:12.04
RUN apt-get update && apt-get install -y python python-pip curl
RUN curl -sSL https://github.com/shykes/helloflask/archive/master.tar.gz | tar -xzv
RUN cd helloflask-master && pip install -r requirements.txt
```

Note that Docker doesn't care *how* dependencies are built - as long
as they can be built by running a Unix command in a container.


Getting started
===============

Docker can be installed on your local machine as well as servers - both
bare metal and virtualized.  It is available as a binary on most modern
Linux systems, or as a VM on Windows, Mac and other systems.

We also offer an [interactive tutorial](http://www.docker.com/tryit/)
for quickly learning the basics of using Docker.

For up-to-date install instructions, see the [Docs](http://docs.docker.com).

Usage examples
==============

Docker can be used to run short-lived commands, long-running daemons
(app servers, databases etc.), interactive shell sessions, etc.

You can find a [list of real-world
examples](http://docs.docker.com/examples/) in the
documentation.

Under the hood
--------------

Under the hood, Docker is built on the following components:

* The
  [cgroup](http://blog.dotcloud.com/kernel-secrets-from-the-paas-garage-part-24-c)
  and
  [namespacing](http://blog.dotcloud.com/under-the-hood-linux-kernels-on-dotcloud-part)
  capabilities of the Linux kernel;
* The [Go](http://golang.org) programming language.

Contributing to Docker
======================

[![GoDoc](https://godoc.org/github.com/docker/docker?status.png)](https://godoc.org/github.com/docker/docker)
[![Travis](https://travis-ci.org/docker/docker.svg?branch=master)](https://travis-ci.org/docker/docker)

Want to hack on Docker? Awesome! There are instructions to get you
started [here](CONTRIBUTING.md).

They are probably not perfect, please let us know if anything feels
wrong or incomplete.

### Legal

*Brought to you courtesy of our legal counsel. For more context,
please see the Notice document.*

Use and transfer of Docker may be subject to certain restrictions by the
United States and other governments.  
It is your responsibility to ensure that your use and/or transfer does not
violate applicable laws. 

For more information, please see http://www.bis.doc.gov


Licensing
=========
Docker is licensed under the Apache License, Version 2.0. See LICENSE for full license text.


Docker: the Linux container runtime
===================================

Docker complements LXC with a high-level API which operates at the process level. It runs unix processes with strong guarantees of isolation and repeatability across servers.

Docker is a great building block for automating distributed systems: large-scale web deployments, database clusters, continuous deployment systems, private PaaS, service-oriented architectures, etc.

<img src="http://bricks.argz.com/bricksfiles/lego/07000/7823/012.jpg"/>

* *Heterogeneous payloads*: any combination of binaries, libraries, configuration files, scripts, virtualenvs, jars, gems, tarballs, you name it. No more juggling between domain-specific tools. Docker can deploy and run them all.
* *Any server*: docker can run on any x64 machine with a modern linux kernel - whether it's a laptop, a bare metal server or a VM. This makes it perfect for multi-cloud deployments.
* *Isolation*: docker isolates processes from each other and from the underlying host, using lightweight containers.
* *Repeatability*: because containers are isolated in their own filesystem, they behave the same regardless of where, when, and alongside what they run.


Notable features
-----------------

* Filesystem isolation: each process container runs in a completely separate root filesystem.
* Resource isolation: system resources like cpu and memory can be allocated differently to each process container, using cgroups.
* Network isolation: each process container runs in its own network namespace, with a virtual interface and IP address of its own.
* Copy-on-write: root filesystems are created using copy-on-write, which makes deployment extremeley fast, memory-cheap and disk-cheap.
* Logging: the standard streams (stdout/stderr/stdin) of each process container are collected and logged for real-time or batch retrieval.
* Change management: changes to a container's filesystem can be committed into a new image and re-used to create more containers. No templating or manual configuration required.
* Interactive shell: docker can allocate a pseudo-tty and attach to the standard input of any container, for example to run a throwaway interactive shell.


Under the hood
--------------

Under the hood, Docker is built on the following components:


* The [cgroup](http://blog.dotcloud.com/kernel-secrets-from-the-paas-garage-part-24-c) and [namespacing](http://blog.dotcloud.com/under-the-hood-linux-kernels-on-dotcloud-part) capabilities of the Linux kernel;
* [AUFS](http://aufs.sourceforge.net/aufs.html), a powerful union filesystem with copy-on-write capabilities;
* The [Go](http://golang.org) programming language;
* [lxc](http://lxc.sourceforge.net/), a set of convenience scripts to simplify the creation of linux containers.


Install instructions
==================

Installing on Ubuntu 12.04 and 12.10
------------------------------------

1. Install dependencies:

    ```bash
    sudo apt-get install lxc wget bsdtar curl
    sudo apt-get install linux-image-extra-`uname -r`
    ```

    The `linux-image-extra` package is needed on standard Ubuntu EC2 AMIs in order to install the aufs kernel module.

2. Install the latest docker binary:

    ```bash
    wget http://get.docker.io/builds/$(uname -s)/$(uname -m)/docker-master.tgz
    tar -xf docker-master.tgz
    ```

3. Run your first container!

    ```bash
    cd docker-master
    sudo ./docker pull base
    sudo ./docker run -i -t base /bin/bash
    ```

    Consider adding docker to your `PATH` for simplicity.

Installing on other Linux distributions
---------------------------------------

Right now, the officially supported distributions are:

* Ubuntu 12.04 (precise LTS)
* Ubuntu 12.10 (quantal)

Docker probably works on other distributions featuring a recent kernel, the AUFS patch, and up-to-date lxc. However this has not been tested.

Some streamlined (but possibly outdated) installation paths' are available from the website: http://docker.io/documentation/ 


Usage examples
==============

Running an interactive shell
----------------------------

```bash
# Download a base image
docker pull base

# Run an interactive shell in the base image,
# allocate a tty, attach stdin and stdout
docker run -i -t base /bin/bash
```


Starting a long-running worker process
--------------------------------------

```bash
# Run docker in daemon mode
(docker -d || echo "Docker daemon already running") &

# Start a very useful long-running process
JOB=$(docker run -d base /bin/sh -c "while true; do echo Hello world; sleep 1; done")

# Collect the output of the job so far
docker logs $JOB

# Kill the job
docker kill $JOB
```


Listing all running containers
------------------------------

```bash
docker ps
```


Expose a service on a TCP port
------------------------------

```bash
# Expose port 4444 of this container, and tell netcat to listen on it
JOB=$(docker run -d -p 4444 base /bin/nc -l -p 4444)

# Which public port is NATed to my container?
PORT=$(docker port $JOB 4444)

# Connect to the public port via the host's public address
echo hello world | nc $(hostname) $PORT

# Verify that the network connection worked
echo "Daemon received: $(docker logs $JOB)"
```

Contributing to Docker
======================

Want to hack on Docker? Awesome! There are instructions to get you started on the website: http://docker.io/documentation/contributing/contributing.html 

They are probably not perfect, please let us know if anything feels wrong or incomplete.


Note
----

We also keep the documentation in this repository. The website documentation is generated using sphinx using these sources.
Please find it under docs/sources/ and read more about it https://github.com/dotcloud/docker/master/docs/README.md

Please feel free to fix / update the documentation and send us pull requests. More tutorials are also welcome.




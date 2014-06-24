## libcontainer - reference implementation for containers

### Note on API changes:

Please bear with us while we work on making the libcontainer API stable and something that we can support long term.  We are currently discussing the API with the community, therefore, if you currently depend on libcontainer please pin your dependency at a specific tag or commit id.  Please join the discussion and help shape the API.

#### Background

libcontainer specifies configuration options for what a container is.  It provides a native Go implementation 
for using Linux namespaces with no external dependencies.  libcontainer provides many convenience functions for working with namespaces, networking, and management.  


#### Container
A container is a self contained directory that is able to run one or more processes without 
affecting the host system.  The directory is usually a full system tree.  Inside the directory
a `container.json` file is placed with the runtime configuration for how the processes 
should be contained and run.  Environment, networking, and different capabilities for the 
process are specified in this file.  The configuration is used for each process executed inside the container.

See the `sample_configs` folder for examples of what the container configuration should look like.

Using this configuration and the current directory holding the rootfs for a process, one can use libcontainer to exec the container. Running the life of the namespace, a `pid` file 
is written to the current directory with the pid of the namespaced process to the external world.  A client can use this pid to wait, kill, or perform other operation with the container.  If a user tries to run a new process inside an existing container with a live namespace, the namespace will be joined by the new process.

You may also specify an alternate root place where the `container.json` file is read and where the `pid` file will be saved.

#### nsinit

`nsinit` is a cli application used as the reference implementation of libcontainer.  It is able to 
spawn or join new containers giving the current directory.  To use `nsinit` cd into a Linux 
rootfs and copy a `container.json` file into the directory with your specified configuration.

To execute `/bin/bash` in the current directory as a container just run:
```bash
nsinit exec /bin/bash
```

If you wish to spawn another process inside the container while your current bash session is 
running just run the exact same command again to get another bash shell or change the command.  If the original process dies, PID 1, all other processes spawned inside the container will also be killed and the namespace will be removed. 

You can identify if a process is running in a container by looking to see if `pid` is in the root of the directory.   
#### Future
See the [roadmap](ROADMAP.md).

## Copyright and license

Code and documentation copyright 2014 Docker, inc. Code released under the Apache 2.0 license.
Docs released under Creative commons.

## Hacking on libcontainer

First of all, please familiarise yourself with the [libcontainer Principles](PRINCIPLES.md).

If you're a *contributor* or aspiring contributor, you should read the [Contributors' Guide](CONTRIBUTORS_GUIDE.md).

If you're a *maintainer* or aspiring maintainer, you should read the [Maintainers' Guide](MAINTAINERS_GUIDE.md) and
"How can I become a maintainer?" in the Contributors' Guide.

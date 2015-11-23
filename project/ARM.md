# ARM support

The ARM support should be considered experimental. It will be extended step by step in the coming weeks.

Building a Docker Development Image works in the same fashion as for Intel platform (x86-64).
Currently we have initial support for 32bit ARMv7 devices.

To work with the Docker Development Image you have to clone the Docker/Docker repo on a supported device.
It needs to have a Docker Engine installed to build the Docker Development Image.

From the root of the Docker/Docker repo one can use make to execute the following make targets:
- make validate
- make binary
- make build
- make bundles
- make default
- make shell
- make

The Makefile does include logic to determine on which OS and architecture the Docker Development Image is built.
Based on OS and architecture it chooses the correct Dockerfile.
For the ARM 32bit architecture it uses `Dockerfile.arm`.

So for example in order to build a Docker binary one has to  
1. clone the Docker/Docker repository on an ARM device `git clone git@github.com:docker/docker.git`  
2. change into the checked out repository with `cd docker`  
3. execute `make binary` to create a Docker Engine binary for ARM  

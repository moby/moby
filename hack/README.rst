This directory contains material helpful for hacking on docker.

make hack
=========

Set up an Ubuntu 12.04 virtual machine for developers including kernel 3.8
go1.1 and buildbot. The environment is setup in a way that can be used through
the usual go workflow and/or the root Makefile. You can either edit on
your host, or inside the VM (using make ssh-dev) and run and test docker
inside the VM.

dependencies: vagrant, virtualbox packages and python package requests


Buildbot
~~~~~~~~

Buildbot is a continuous integration system designed to automate the
build/test cycle. By automatically rebuilding and testing the tree each time
something has changed, build problems are pinpointed quickly, before other
developers are inconvenienced by the failure.

When running 'make hack' at the docker root directory, it spawns a virtual
machine in the background running a buildbot instance and adds a git
post-commit hook that automatically run docker tests for you each time you
commit in your local docker repository.

You can check your buildbot instance at http://192.168.33.21:8010/waterfall

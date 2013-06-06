:title: Setting Up a Dev Environment
:description: Guides on how to contribute to docker
:keywords: Docker, documentation, developers, contributing, dev environment

Setting Up a Dev Environment
============================

Instructions that have been verified to work on Ubuntu Precise 12.04 (LTS) (64-bit),


Dependencies
------------

**Linux kernel 3.8**

Due to a bug in LXC docker works best on the 3.8 kernel. Precise comes with a 3.2 kernel, so we need to upgrade it. The kernel we install comes with AUFS built in.


.. code-block:: bash

   # install the backported kernel
   sudo apt-get update && sudo apt-get install linux-image-generic-lts-raring

   # reboot
   sudo reboot


Installation
------------

.. code-block:: bash
		
    sudo apt-get install python-software-properties
    sudo add-apt-repository ppa:gophers/go
    sudo apt-get update
    sudo apt-get -y install lxc wget bsdtar curl golang-stable git

    export GOPATH=~/go/
    export PATH=$GOPATH/bin:$PATH

    mkdir -p $GOPATH/src/github.com/dotcloud
    cd $GOPATH/src/github.com/dotcloud
    git clone git://github.com/dotcloud/docker.git
    cd docker

    go get -v github.com/dotcloud/docker/...
    go install -v github.com/dotcloud/docker/...


Then run the docker daemon,

.. code-block:: bash

    sudo $GOPATH/bin/docker -d


Run the ``go install`` command (above) to recompile docker.

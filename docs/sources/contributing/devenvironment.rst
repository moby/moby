:title: Setting up a dev environment
:description: Guides on how to contribute to docker
:keywords: Docker, documentation, developers, contributing, dev environment

Setting up a dev environment
============================

Instructions that have been verified to work on Ubuntu 12.10,

.. code-block:: bash

    sudo apt-get -y install lxc wget bsdtar curl golang git

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

:title: Setting Up a Dev Environment
:description: Guides on how to contribute to docker
:keywords: Docker, documentation, developers, contributing, dev environment

Setting Up a Dev Environment
============================

The following instructions have been tested on Ubuntu 13.04 64-bit.

.. code-block:: console


   $ # Install Go 1.1.2 or later.
   $ cd
   $ export GOPATH=~/docker
   $ export GOROOT=~/go
   $ export PATH=$GOPATH/bin:$GOROOT/bin:$PATH
   $ wget -O - -q http://go.googlecode.com/files/go1.1.2.linux-amd64.tar.gz | tar xz

   $ # Install Docker and the rest of its dependencies.
   $ sudo apt-get -y install git lxc mercurial > /dev/null
   $ git clone https://github.com/dotcloud/docker.git ~/docker/github.com/dotcloud/docker
   $ go get github.com/dotcloud/docker/...
   $ go install github.com/dotcloud/docker/...
   $ sudo ln -s ~/docker/bin/docker /usr/local/bin/docker

   $ # To...
   $ # ...run the tests (don't forget to replace "david" with your user):
   $ sudo su - root
   $ export GOPATH=~david/docker
   $ export GOROOT=~david/go
   $ export PATH=$GOPATH/bin:$GOROOT/bin:$PATH
   $ go test -v github.com/dotcloud/docker
   ...
   ok   github.com/dotcloud/docker  45.570s
   $ # ...start Docker:
   $ sudo docker -d > /dev/null 2>&1 &


:title: Setting Up a Dev Environment
:description: Guides on how to contribute to docker
:keywords: Docker, documentation, developers, contributing, dev environment

Setting Up a Dev Environment
============================

The following instructions have been tested on Ubuntu 13.04 64-bit.

.. code-block:: console

   $ # Install Go 1.1.2.
   $ cd
   $ export GOPATH=~/docker
   $ export GOROOT=~/go
   $ export PATH=$GOPATH/bin:$GOROOT/bin:$PATH
   $ wget -O - -q http://go.googlecode.com/files/go1.1.2.linux-amd64.tar.gz | tar xz

   $ # Install the rest of Docker's dependencies.
   $ sudo apt-get -y install git lxc mercurial > /dev/null
   $ PKG=code.google.com/p/go.net   REV=84a4013f96e0; hg clone -q https://$PKG $GOPATH/src/$PKG;  cd $GOPATH/src/$PKG; hg checkout -q $REV
   $ PKG=github.com/dotcloud/tar    REV=d06045a6d9;   git clone -q https://$PKG $GOPATH/src/$PKG; cd $GOPATH/src/$PKG; git checkout -q $REV
   $ PKG=github.com/gorilla/context REV=708054d61e5;  git clone -q https://$PKG $GOPATH/src/$PKG; cd $GOPATH/src/$PKG; git checkout -q $REV
   $ PKG=github.com/gorilla/mux     REV=9b36453141c;  git clone -q https://$PKG $GOPATH/src/$PKG; cd $GOPATH/src/$PKG; git checkout -q $REV
   $ PKG=github.com/kr/pty          REV=27435c699;    git clone -q https://$PKG $GOPATH/src/$PKG; cd $GOPATH/src/$PKG; git checkout -q $REV

   $ # Install Docker.
   $ git clone -q https://github.com/dotcloud/docker $GOPATH/src/github.com/dotcloud/docker
   $ go install github.com/dotcloud/docker/...
   $ sudo ln -s $GOPATH/bin/docker /usr/local/bin/docker

   $ # That's it! To...
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

#!/bin/bash

if [  $# -gt 0 ] ; then
    ZK_VERSION="$1"
else
    ZK_VERSION="3.4.6"
fi

wget "http://mirrors.ukfast.co.uk/sites/ftp.apache.org/zookeeper/stable/zookeeper-${ZK_VERSION}.tar.gz"
tar -xvf "zookeeper-${ZK_VERSION}.tar.gz"
mv zookeeper-$ZK_VERSION zk
mv ./zk/conf/zoo_sample.cfg ./zk/conf/zoo.cfg

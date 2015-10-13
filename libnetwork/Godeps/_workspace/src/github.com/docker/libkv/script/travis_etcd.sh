#!/bin/bash

if [  $# -gt 0 ] ; then
    ETCD_VERSION="$1"
else
    ETCD_VERSION="2.2.0"
fi

curl -L https://github.com/coreos/etcd/releases/download/v$ETCD_VERSION/etcd-v$ETCD_VERSION-linux-amd64.tar.gz -o etcd-v$ETCD_VERSION-linux-amd64.tar.gz
tar xzvf etcd-v$ETCD_VERSION-linux-amd64.tar.gz
mv etcd-v$ETCD_VERSION-linux-amd64 etcd

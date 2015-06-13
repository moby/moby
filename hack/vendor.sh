#!/usr/bin/env bash
set -e

cd "$(dirname "$BASH_SOURCE")/.."
source 'hack/.vendor-helpers.sh'

# the following lines are in sorted order, FYI
clone git github.com/Sirupsen/logrus v0.8.2 # logrus is a common dependency among multiple deps
clone git github.com/docker/libtrust 230dfd18c232
clone git github.com/go-check/check 64131543e7896d5bcc6bd5a76287eb75ea96c673
clone git github.com/gorilla/context 14f550f51a
clone git github.com/gorilla/mux e444e69cbd
clone git github.com/kr/pty 5cf931ef8f
clone git github.com/mistifyio/go-zfs v2.1.1
clone git github.com/tchap/go-patricia v2.1.0
clone hg code.google.com/p/go.net 84a4013f96e0
clone hg code.google.com/p/gosqlite 74691fb6f837

#get libnetwork packages
clone git github.com/docker/libnetwork 3be488927db8d719568917203deddd630a194564
clone git github.com/docker/libkv e8cde779d58273d240c1eff065352a6cd67027dd
clone git github.com/vishvananda/netns 5478c060110032f972e86a1f844fdb9a2f008f2c
clone git github.com/vishvananda/netlink 8eb64238879fed52fd51c5b30ad20b928fb4c36c
clone git github.com/BurntSushi/toml f706d00e3de6abe700c994cdd545a1a4915af060
clone git github.com/samuel/go-zookeeper d0e0d8e11f318e000a8cc434616d69e329edc374
clone git github.com/coreos/go-etcd v2.0.0
clone git github.com/hashicorp/consul v0.5.2

# get distribution packages
clone git github.com/docker/distribution b9eeb328080d367dbde850ec6e94f1e4ac2b5efe

clone git github.com/docker/libcontainer v2.1.1
# libcontainer deps (see src/github.com/docker/libcontainer/update-vendor.sh)
clone git github.com/coreos/go-systemd v2
clone git github.com/godbus/dbus v2
clone git github.com/syndtr/gocapability 66ef2aa7a23ba682594e2b6f74cf40c0692b49fb
clone git github.com/golang/protobuf 655cdfa588ea
clone git github.com/Graylog2/go-gelf 6c62a85f1d47a67f2a5144c0e745b325889a8120

clean

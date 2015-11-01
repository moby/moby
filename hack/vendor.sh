#!/usr/bin/env bash
set -e

cd "$(dirname "$BASH_SOURCE")/.."
rm -rf vendor/
source 'hack/.vendor-helpers.sh'

# the following lines are in sorted order, FYI
clone git github.com/Azure/go-ansiterm 70b2c90b260171e829f1ebd7c17f600c11858dbe
clone git github.com/Sirupsen/logrus v0.8.2 # logrus is a common dependency among multiple deps
clone git github.com/docker/libtrust 9cbd2a1374f46905c68a4eb3694a130610adc62a
clone git github.com/go-check/check 64131543e7896d5bcc6bd5a76287eb75ea96c673
clone git github.com/gorilla/context 14f550f51a
clone git github.com/gorilla/mux e444e69cbd
clone git github.com/kr/pty 5cf931ef8f
clone git github.com/mattn/go-sqlite3 v1.1.0
clone git github.com/microsoft/hcsshim 7f646aa6b26bcf90caee91e93cde4a80d0d8a83e
clone git github.com/mistifyio/go-zfs v2.1.1
clone git github.com/tchap/go-patricia v2.1.0
clone git golang.org/x/net 3cffabab72adf04f8e3b01c5baf775361837b5fe https://github.com/golang/net.git

#get libnetwork packages
clone git github.com/docker/libnetwork e7719596c01a83f9ef24d33e9d609a64acacd7b8
clone git github.com/armon/go-metrics eb0af217e5e9747e41dd5303755356b62d28e3ec
clone git github.com/hashicorp/go-msgpack 71c2886f5a673a35f909803f38ece5810165097b
clone git github.com/hashicorp/memberlist 9a1e242e454d2443df330bdd51a436d5a9058fc4
clone git github.com/hashicorp/serf 7151adcef72687bf95f451a2e0ba15cb19412bf2
clone git github.com/docker/libkv c2aac5dbbaa5c872211edea7c0f32b3bd67e7410
clone git github.com/vishvananda/netns 604eaf189ee867d8c147fafc28def2394e878d25
clone git github.com/vishvananda/netlink 4b5dce31de6d42af5bb9811c6d265472199e0fec
clone git github.com/BurntSushi/toml f706d00e3de6abe700c994cdd545a1a4915af060
clone git github.com/samuel/go-zookeeper d0e0d8e11f318e000a8cc434616d69e329edc374
clone git github.com/deckarep/golang-set ef32fa3046d9f249d399f98ebaf9be944430fd1d
clone git github.com/coreos/etcd v2.2.0
fix_rewritten_imports github.com/coreos/etcd
clone git github.com/ugorji/go 5abd4e96a45c386928ed2ca2a7ef63e2533e18ec
clone git github.com/hashicorp/consul v0.5.2
clone git github.com/boltdb/bolt v1.0

# get graph and distribution packages
clone git github.com/docker/distribution 20c4b7a1805a52753dfd593ee1cc35558722a0ce # docker/1.9 branch
clone git github.com/vbatts/tar-split v0.9.10

clone git github.com/docker/notary 089d8450d8928aa1c58fd03f09cabbde9bcb4590
clone git github.com/endophage/gotuf 2df1c8e0a7b7e10ae2113bf37aaa1bf1c1de8cc5
clone git github.com/jfrazelle/go 6e461eb70cb4187b41a84e9a567d7137bdbe0f16
clone git github.com/agl/ed25519 d2b94fd789ea21d12fac1a4443dd3a3f79cda72c

clone git github.com/opencontainers/runc 6c198ae2d065c37f44316e0de3df7f3b88950923 # libcontainer
# libcontainer deps (see src/github.com/opencontainers/runc/Godeps/Godeps.json)
clone git github.com/coreos/go-systemd v3
clone git github.com/godbus/dbus v2
clone git github.com/syndtr/gocapability 66ef2aa7a23ba682594e2b6f74cf40c0692b49fb
clone git github.com/golang/protobuf 655cdfa588ea
clone git github.com/Graylog2/go-gelf 6c62a85f1d47a67f2a5144c0e745b325889a8120

clone git github.com/fluent/fluent-logger-golang v1.0.0
# fluent-logger-golang deps
clone git github.com/philhofer/fwd 899e4efba8eaa1fea74175308f3fae18ff3319fa
clone git github.com/tinylib/msgp 75ee40d2601edf122ef667e2a07d600d4c44490c

# fsnotify
clone git gopkg.in/fsnotify.v1 v1.2.0

# awslogs deps
clone git github.com/aws/aws-sdk-go v0.7.1
clone git github.com/vaughan0/go-ini a98ad7ee00ec53921f08832bc06ecf7fd600e6a1

clean

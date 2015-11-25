#!/usr/bin/env bash
set -e

cd "$(dirname "$BASH_SOURCE")/.."
rm -rf vendor/
source 'hack/.vendor-helpers.sh'

# the following lines are in sorted order, FYI
clone git github.com/Azure/go-ansiterm 70b2c90b260171e829f1ebd7c17f600c11858dbe
clone git github.com/Sirupsen/logrus v0.8.2 # logrus is a common dependency among multiple deps
clone git github.com/docker/libtrust 9cbd2a1374f46905c68a4eb3694a130610adc62a
clone git github.com/go-check/check 11d3bc7aa68e238947792f30573146a3231fc0f1
clone git github.com/gorilla/context 14f550f51a
clone git github.com/gorilla/mux e444e69cbd
clone git github.com/kr/pty 5cf931ef8f
clone git github.com/mattn/go-shellwords v1.0.0
clone git github.com/mattn/go-sqlite3 v1.1.0
clone git github.com/microsoft/hcsshim de43b42b5ce14dfdcbeedb0628b0032174d89caa
clone git github.com/mistifyio/go-zfs v2.1.1
clone git github.com/tchap/go-patricia v2.1.0
clone git github.com/vdemeester/shakers 3c10293ce22b900c27acad7b28656196fcc2f73b
clone git golang.org/x/net 47990a1ba55743e6ef1affd3a14e5bac8553615d https://github.com/golang/net.git

#get libnetwork packages
clone git github.com/docker/libnetwork 04cc1fa0a89f8c407b7be8cab883d4b17531ea7d
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
clone git github.com/boltdb/bolt v1.1.0

# get graph and distribution packages
clone git github.com/docker/distribution c6c9194e9c6097f84b0ff468a741086ff7704aa3
clone git github.com/vbatts/tar-split v0.9.10

clone git github.com/docker/notary 45de2828b5e0083bfb4e9a5a781eddb05e2ef9d0
clone git google.golang.org/grpc 174192fc93efcb188fc8f46ca447f0da606b6885 https://github.com/grpc/grpc-go.git
clone git github.com/miekg/pkcs11 80f102b5cac759de406949c47f0928b99bd64cdf
clone git github.com/jfrazelle/go v1.5.1-1
clone git github.com/agl/ed25519 d2b94fd789ea21d12fac1a4443dd3a3f79cda72c

clone git github.com/opencontainers/runc v0.0.5 # libcontainer
# libcontainer deps (see src/github.com/opencontainers/runc/Godeps/Godeps.json)
clone git github.com/coreos/go-systemd v4
clone git github.com/godbus/dbus v3
clone git github.com/syndtr/gocapability 2c00daeb6c3b45114c80ac44119e7b8801fdd852
clone git github.com/golang/protobuf f7137ae6b19afbfd61a94b746fda3b3fe0491874

# gelf logging driver deps
clone git github.com/Graylog2/go-gelf 6c62a85f1d47a67f2a5144c0e745b325889a8120

clone git github.com/fluent/fluent-logger-golang v1.0.0
# fluent-logger-golang deps
clone git github.com/philhofer/fwd 899e4efba8eaa1fea74175308f3fae18ff3319fa
clone git github.com/tinylib/msgp 75ee40d2601edf122ef667e2a07d600d4c44490c

# fsnotify
clone git gopkg.in/fsnotify.v1 v1.2.0

# awslogs deps
clone git github.com/aws/aws-sdk-go v0.9.9
clone git github.com/vaughan0/go-ini a98ad7ee00ec53921f08832bc06ecf7fd600e6a1

clean

#!/usr/bin/env bash
set -e

# this script is used to update vendored dependencies
#
# Usage:
# vendor.sh revendor all dependencies
# vendor.sh github.com/docker/engine-api revendor only the engine-api dependency.
# vendor.sh github.com/docker/engine-api v0.3.3 vendor only engine-api at the specified tag/commit.
# vendor.sh git github.com/docker/engine-api v0.3.3 is the same but specifies the VCS for cases where the VCS is something else than git
# vendor.sh git golang.org/x/sys eb2c74142fd19a79b3f237334c7384d5167b1b46 https://github.com/golang/sys.git vendor only golang.org/x/sys downloading from the specified URL

cd "$(dirname "$BASH_SOURCE")/.."
source 'hack/.vendor-helpers.sh'

case $# in
0)
	rm -rf vendor/
	;;
# If user passed arguments to the script
1)
	eval "$(grep -E "^clone [^ ]+ $1" "$0")"
	clean
	exit 0
	;;
2)
	rm -rf "vendor/src/$1"
	clone git "$1" "$2"
	clean
	exit 0
	;;
[34])
	rm -rf "vendor/src/$2"
	clone "$@"
	clean
	exit 0
	;;
*)
	>&2 echo "error: unexpected parameters"
	exit 1
	;;
esac

# the following lines are in sorted order, FYI
clone git github.com/Azure/go-ansiterm 388960b655244e76e24c75f48631564eaefade62
clone git github.com/Microsoft/hcsshim v0.3.1
clone git github.com/Microsoft/go-winio v0.3.4
clone git github.com/Sirupsen/logrus v0.10.0 # logrus is a common dependency among multiple deps
clone git github.com/docker/libtrust 9cbd2a1374f46905c68a4eb3694a130610adc62a
clone git github.com/go-check/check 03a4d9dcf2f92eae8e90ed42aa2656f63fdd0b14 https://github.com/cpuguy83/check.git
clone git github.com/gorilla/context 14f550f51a
clone git github.com/gorilla/mux e444e69cbd
clone git github.com/kr/pty 5cf931ef8f
clone git github.com/mattn/go-shellwords v1.0.0
clone git github.com/mattn/go-sqlite3 v1.1.0
clone git github.com/tchap/go-patricia v2.1.0
clone git github.com/vdemeester/shakers 24d7f1d6a71aa5d9cbe7390e4afb66b7eef9e1b3
# forked golang.org/x/net package includes a patch for lazy loading trace templates
clone git golang.org/x/net 2beffdc2e92c8a3027590f898fe88f69af48a3f8 https://github.com/tonistiigi/net.git
clone git golang.org/x/sys eb2c74142fd19a79b3f237334c7384d5167b1b46 https://github.com/golang/sys.git
clone git github.com/docker/go-units 651fc226e7441360384da338d0fd37f2440ffbe3
clone git github.com/docker/go-connections v0.2.0
clone git github.com/docker/engine-api 772250a752e34cacaeef7c92b8e0ddf43450b629
clone git github.com/RackSec/srslog 259aed10dfa74ea2961eddd1d9847619f6e98837
clone git github.com/imdario/mergo 0.2.1

#get libnetwork packages
clone git github.com/docker/libnetwork b66c0385f30c6aa27b2957ed1072682c19a0b0b4
clone git github.com/docker/go-events 2e7d352816128aa84f4d29b2a21d400133701a0d
clone git github.com/armon/go-radix e39d623f12e8e41c7b5529e9a9dd67a1e2261f80
clone git github.com/armon/go-metrics eb0af217e5e9747e41dd5303755356b62d28e3ec
clone git github.com/hashicorp/go-msgpack 71c2886f5a673a35f909803f38ece5810165097b
clone git github.com/hashicorp/memberlist 88ac4de0d1a0ca6def284b571342db3b777a4c37
clone git github.com/hashicorp/go-multierror fcdddc395df1ddf4247c69bd436e84cfa0733f7e
clone git github.com/hashicorp/serf 598c54895cc5a7b1a24a398d635e8c0ea0959870
clone git github.com/docker/libkv 7283ef27ed32fe267388510a91709b307bb9942c
clone git github.com/vishvananda/netns 604eaf189ee867d8c147fafc28def2394e878d25
clone git github.com/vishvananda/netlink 631962935bff4f3d20ff32a72e8944f6d2836a26
clone git github.com/BurntSushi/toml f706d00e3de6abe700c994cdd545a1a4915af060
clone git github.com/samuel/go-zookeeper d0e0d8e11f318e000a8cc434616d69e329edc374
clone git github.com/deckarep/golang-set ef32fa3046d9f249d399f98ebaf9be944430fd1d
clone git github.com/coreos/etcd v2.3.2
fix_rewritten_imports github.com/coreos/etcd
clone git github.com/ugorji/go f1f1a805ed361a0e078bb537e4ea78cd37dcf065
clone git github.com/hashicorp/consul v0.5.2
clone git github.com/boltdb/bolt v1.2.1
clone git github.com/miekg/dns 75e6e86cc601825c5dbcd4e0c209eab180997cd7

# get graph and distribution packages
clone git github.com/docker/distribution 5bbf65499960b184fe8e0f045397375e1a6722b8
clone git github.com/vbatts/tar-split v0.9.11

# get go-zfs packages
clone git github.com/mistifyio/go-zfs 22c9b32c84eb0d0c6f4043b6e90fc94073de92fa
clone git github.com/pborman/uuid v1.0

# get desired notary commit, might also need to be updated in Dockerfile
clone git github.com/docker/notary v0.3.0

clone git google.golang.org/grpc ab0be5212fb225475f2087566eded7da5d727960 https://github.com/grpc/grpc-go.git
clone git github.com/miekg/pkcs11 df8ae6ca730422dba20c768ff38ef7d79077a59f
clone git github.com/docker/go v1.5.1-1-1-gbaf439e
clone git github.com/agl/ed25519 d2b94fd789ea21d12fac1a4443dd3a3f79cda72c

clone git github.com/opencontainers/runc d49ece5a83da3dcb820121d6850e2b61bd0a5fbe # libcontainer
clone git github.com/opencontainers/specs f955d90e70a98ddfb886bd930ffd076da9b67998 # specs
clone git github.com/seccomp/libseccomp-golang 60c9953736798c4a04e90d0f3da2f933d44fd4c4
# libcontainer deps (see src/github.com/opencontainers/runc/Godeps/Godeps.json)
clone git github.com/coreos/go-systemd v4
clone git github.com/godbus/dbus v4.0.0
clone git github.com/syndtr/gocapability 2c00daeb6c3b45114c80ac44119e7b8801fdd852
clone git github.com/golang/protobuf 3c84672111d91bb5ac31719e112f9f7126a0e26e

# gelf logging driver deps
clone git github.com/Graylog2/go-gelf aab2f594e4585d43468ac57287b0dece9d806883

clone git github.com/fluent/fluent-logger-golang v1.1.0
# fluent-logger-golang deps
clone git github.com/philhofer/fwd 899e4efba8eaa1fea74175308f3fae18ff3319fa
clone git github.com/tinylib/msgp 75ee40d2601edf122ef667e2a07d600d4c44490c

# fsnotify
clone git gopkg.in/fsnotify.v1 v1.2.11

# awslogs deps
clone git github.com/aws/aws-sdk-go v1.1.30
clone git github.com/go-ini/ini 060d7da055ba6ec5ea7a31f116332fe5efa04ce0
clone git github.com/jmespath/go-jmespath 0b12d6b521d83fc7f755e7cfc1b1fbdd35a01a74

# gcplogs deps
clone git golang.org/x/oauth2 2baa8a1b9338cf13d9eeb27696d761155fa480be https://github.com/golang/oauth2.git
clone git google.golang.org/api dc6d2353af16e2a2b0ff6986af051d473a4ed468 https://code.googlesource.com/google-api-go-client
clone git google.golang.org/cloud dae7e3d993bc3812a2185af60552bb6b847e52a0 https://code.googlesource.com/gocloud

# native credentials
clone git github.com/docker/docker-credential-helpers v0.3.0

# containerd
clone git github.com/docker/containerd cf554d59dd96e459544748290eb9167f4bcde509

# cli
clone git github.com/spf13/cobra 75205f23b3ea70dc7ae5e900d074e010c23c37e9 https://github.com/dnephin/cobra.git
clone git github.com/spf13/pflag cb88ea77998c3f024757528e3305022ab50b43be
clone git github.com/inconshreveable/mousetrap 76626ae9c91c4f2a10f34cad8ce83ea42c93bb75

clean

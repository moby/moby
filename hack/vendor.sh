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
clone git github.com/Microsoft/hcsshim v0.3.4
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
clone git github.com/docker/go-connections fa2850ff103453a9ad190da0df0af134f0314b3d
clone git github.com/docker/engine-api 4eca04ae18f4f93f40196a17b9aa6e11262a7269
clone git github.com/RackSec/srslog 456df3a81436d29ba874f3590eeeee25d666f8a5
clone git github.com/imdario/mergo 0.2.1

#get libnetwork packages
clone git github.com/docker/libnetwork 09bc1d0839e32843828ced297ad634822a48c38b
clone git github.com/docker/go-events afb2b9f2c23f33ada1a22b03651775fdc65a5089
clone git github.com/armon/go-radix e39d623f12e8e41c7b5529e9a9dd67a1e2261f80
clone git github.com/armon/go-metrics eb0af217e5e9747e41dd5303755356b62d28e3ec
clone git github.com/hashicorp/go-msgpack 71c2886f5a673a35f909803f38ece5810165097b
clone git github.com/hashicorp/memberlist 88ac4de0d1a0ca6def284b571342db3b777a4c37
clone git github.com/hashicorp/go-multierror fcdddc395df1ddf4247c69bd436e84cfa0733f7e
clone git github.com/hashicorp/serf 598c54895cc5a7b1a24a398d635e8c0ea0959870
clone git github.com/docker/libkv 1d8431073ae03cdaedb198a89722f3aab6d418ef
clone git github.com/vishvananda/netns 604eaf189ee867d8c147fafc28def2394e878d25
clone git github.com/vishvananda/netlink 17ea11b5a11c5614597c65a671105e8ee58c4d04
clone git github.com/BurntSushi/toml f706d00e3de6abe700c994cdd545a1a4915af060
clone git github.com/samuel/go-zookeeper d0e0d8e11f318e000a8cc434616d69e329edc374
clone git github.com/deckarep/golang-set ef32fa3046d9f249d399f98ebaf9be944430fd1d
clone git github.com/coreos/etcd v2.3.2
fix_rewritten_imports github.com/coreos/etcd
clone git github.com/ugorji/go f1f1a805ed361a0e078bb537e4ea78cd37dcf065
clone git github.com/hashicorp/consul v0.5.2
clone git github.com/boltdb/bolt fff57c100f4dea1905678da7e90d92429dff2904
clone git github.com/miekg/dns 75e6e86cc601825c5dbcd4e0c209eab180997cd7

# get graph and distribution packages
clone git github.com/docker/distribution 07f32ac1831ed0fc71960b7da5d6bb83cb6881b5
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

clone git github.com/opencontainers/runc 50a19c6ff828c58e5dab13830bd3dacde268afe5 https://github.com/docker/runc.git # libcontainer
clone git github.com/opencontainers/specs 1c7c27d043c2a5e513a44084d2b10d77d1402b8c # specs
clone git github.com/seccomp/libseccomp-golang 32f571b70023028bd57d9288c20efbcb237f3ce0
# libcontainer deps (see src/github.com/opencontainers/runc/Godeps/Godeps.json)
clone git github.com/coreos/go-systemd v4
clone git github.com/godbus/dbus v4.0.0
clone git github.com/syndtr/gocapability 2c00daeb6c3b45114c80ac44119e7b8801fdd852
clone git github.com/golang/protobuf 3c84672111d91bb5ac31719e112f9f7126a0e26e

# gelf logging driver deps
clone git github.com/Graylog2/go-gelf aab2f594e4585d43468ac57287b0dece9d806883

clone git github.com/fluent/fluent-logger-golang v1.2.1
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
clone git github.com/docker/containerd 2a5e70cbf65457815ee76b7e5dd2a01292d9eca8
clone git github.com/tonistiigi/fifo fe870ccf293940774c2b44e23f6c71fff8f7547d

# cluster
clone git github.com/docker/swarmkit 0cf248feec033f46dc09db40d69fd5128082b79a
clone git github.com/golang/mock bd3c8e81be01eef76d4b503f5e687d2d1354d2d9
clone git github.com/gogo/protobuf 43a2e0b1c32252bfbbdf81f7faa7a88fb3fa4028
clone git github.com/cloudflare/cfssl b895b0549c0ff676f92cf09ba971ae02bb41367b
clone git github.com/google/certificate-transparency 025a5cab06f6a819c455d9fdc9e2a1b6d0982284
clone git golang.org/x/crypto 3fbbcd23f1cb824e69491a5930cfeff09b12f4d2 https://github.com/golang/crypto.git
clone git github.com/mreiferson/go-httpclient 63fe23f7434723dc904c901043af07931f293c47
clone git github.com/hashicorp/go-memdb 98f52f52d7a476958fa9da671354d270c50661a7
clone git github.com/hashicorp/go-immutable-radix 8e8ed81f8f0bf1bdd829593fdd5c29922c1ea990
clone git github.com/hashicorp/golang-lru a0d98a5f288019575c6d1f4bb1573fef2d1fcdc4
clone git github.com/coreos/pkg 2c77715c4df99b5420ffcae14ead08f52104065d
clone git github.com/pivotal-golang/clock 3fd3c1944c59d9742e1cd333672181cd1a6f9fa0
clone git github.com/prometheus/client_golang e51041b3fa41cece0dca035740ba6411905be473
clone git github.com/beorn7/perks b965b613227fddccbfffe13eae360ed3fa822f8d
clone git github.com/prometheus/client_model fa8ad6fec33561be4280a8f0514318c79d7f6cb6
clone git github.com/prometheus/common ffe929a3f4c4faeaa10f2b9535c2b1be3ad15650
clone git github.com/prometheus/procfs 454a56f35412459b5e684fd5ec0f9211b94f002a
clone hg bitbucket.org/ww/goautoneg 75cd24fc2f2c2a2088577d12123ddee5f54e0675
clone git github.com/matttproud/golang_protobuf_extensions fc2b8d3a73c4867e51861bbdd5ae3c1f0869dd6a
clone git github.com/pkg/errors 01fa4104b9c248c8945d14d9f128454d5b28d595

# cli
clone git github.com/spf13/cobra 75205f23b3ea70dc7ae5e900d074e010c23c37e9 https://github.com/dnephin/cobra.git
clone git github.com/spf13/pflag cb88ea77998c3f024757528e3305022ab50b43be
clone git github.com/inconshreveable/mousetrap 76626ae9c91c4f2a10f34cad8ce83ea42c93bb75
clone git github.com/flynn-archive/go-shlex 3f9db97f856818214da2e1057f8ad84803971cff

clean

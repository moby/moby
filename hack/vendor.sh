#!/usr/bin/env bash
set -e

# this script is used to update vendored dependencies
#
# Usage:
# vendor.sh revendor all dependencies
# vendor.sh github.com/docker/libkv revendor only the libkv dependency.
# vendor.sh github.com/docker/libkv v0.2.1 vendor only libkv at the specified tag/commit.
# vendor.sh git github.com/docker/libkv v0.2.1 is the same but specifies the VCS for cases where the VCS is something else than git
# vendor.sh git golang.org/x/sys eb2c74142fd19a79b3f237334c7384d5167b1b46 https://github.com/golang/sys.git vendor only golang.org/x/sys downloading from the specified URL

cd "$(dirname "$BASH_SOURCE")/.."
source 'hack/.vendor-helpers.sh'

case $# in
0)
	rm -rf vendor/
	;;
# If user passed arguments to the script
1)
	path="$PWD/hack/vendor.sh"
	if ! cloneGrep="$(grep -E "^clone [^ ]+ $1" "$path")"; then
		echo >&2 "error: failed to find 'clone ... $1' in $path"
		exit 1
	fi
	eval "$cloneGrep"
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
clone git github.com/Microsoft/hcsshim v0.5.1
clone git github.com/Microsoft/go-winio v0.3.5
clone git github.com/Sirupsen/logrus v0.10.0 # logrus is a common dependency among multiple deps
clone git github.com/docker/libtrust 9cbd2a1374f46905c68a4eb3694a130610adc62a
clone git github.com/go-check/check 4ed411733c5785b40214c70bce814c3a3a689609 https://github.com/cpuguy83/check.git
clone git github.com/gorilla/context v1.1
clone git github.com/gorilla/mux v1.1
clone git github.com/kr/pty 5cf931ef8f
clone git github.com/mattn/go-shellwords v1.0.0
clone git github.com/mattn/go-sqlite3 v1.1.0
clone git github.com/tchap/go-patricia v2.2.6
clone git github.com/vdemeester/shakers 24d7f1d6a71aa5d9cbe7390e4afb66b7eef9e1b3
# forked golang.org/x/net package includes a patch for lazy loading trace templates
clone git golang.org/x/net 2beffdc2e92c8a3027590f898fe88f69af48a3f8 https://github.com/tonistiigi/net.git
clone git golang.org/x/sys eb2c74142fd19a79b3f237334c7384d5167b1b46 https://github.com/golang/sys.git
clone git github.com/docker/go-units f2145db703495b2e525c59662db69a7344b00bb8
clone git github.com/docker/go-connections 988efe982fdecb46f01d53465878ff1f2ff411ce

clone git github.com/RackSec/srslog 365bf33cd9acc21ae1c355209865f17228ca534e
clone git github.com/imdario/mergo 0.2.1

#get libnetwork packages
clone git github.com/docker/libnetwork 04025f2a2eebb0d091883e55980dc6916d36842d
clone git github.com/docker/go-events 18b43f1bc85d9cdd42c05a6cd2d444c7a200a894
clone git github.com/armon/go-radix e39d623f12e8e41c7b5529e9a9dd67a1e2261f80
clone git github.com/armon/go-metrics eb0af217e5e9747e41dd5303755356b62d28e3ec
clone git github.com/hashicorp/go-msgpack 71c2886f5a673a35f909803f38ece5810165097b
clone git github.com/hashicorp/memberlist 88ac4de0d1a0ca6def284b571342db3b777a4c37
clone git github.com/hashicorp/go-multierror fcdddc395df1ddf4247c69bd436e84cfa0733f7e
clone git github.com/hashicorp/serf 598c54895cc5a7b1a24a398d635e8c0ea0959870
clone git github.com/docker/libkv v0.2.1
clone git github.com/vishvananda/netns 604eaf189ee867d8c147fafc28def2394e878d25
clone git github.com/vishvananda/netlink e73bad418fd727ed3a02830b1af1ad0283a1de6c
clone git github.com/BurntSushi/toml f706d00e3de6abe700c994cdd545a1a4915af060
clone git github.com/samuel/go-zookeeper d0e0d8e11f318e000a8cc434616d69e329edc374
clone git github.com/deckarep/golang-set ef32fa3046d9f249d399f98ebaf9be944430fd1d
clone git github.com/coreos/etcd 3a49cbb769ebd8d1dd25abb1e83386e9883a5707
clone git github.com/ugorji/go f1f1a805ed361a0e078bb537e4ea78cd37dcf065
clone git github.com/hashicorp/consul v0.5.2
clone git github.com/boltdb/bolt fff57c100f4dea1905678da7e90d92429dff2904
clone git github.com/miekg/dns 75e6e86cc601825c5dbcd4e0c209eab180997cd7

# get graph and distribution packages
clone git github.com/docker/distribution 77b9d2997abcded79a5314970fe69a44c93c25fb
clone git github.com/vbatts/tar-split v0.10.1

# get go-zfs packages
clone git github.com/mistifyio/go-zfs 22c9b32c84eb0d0c6f4043b6e90fc94073de92fa
clone git github.com/pborman/uuid v1.0

# get desired notary commit, might also need to be updated in Dockerfile
clone git github.com/docker/notary v0.3.0

clone git google.golang.org/grpc v1.0.1-GA https://github.com/grpc/grpc-go.git
clone git github.com/miekg/pkcs11 df8ae6ca730422dba20c768ff38ef7d79077a59f
clone git github.com/docker/go v1.5.1-1-1-gbaf439e
clone git github.com/agl/ed25519 d2b94fd789ea21d12fac1a4443dd3a3f79cda72c

clone git github.com/opencontainers/runc 02f8fa7863dd3f82909a73e2061897828460d52f # libcontainer
clone git github.com/opencontainers/runtime-spec 1c7c27d043c2a5e513a44084d2b10d77d1402b8c # specs
clone git github.com/seccomp/libseccomp-golang 32f571b70023028bd57d9288c20efbcb237f3ce0
# libcontainer deps (see src/github.com/opencontainers/runc/Godeps/Godeps.json)
clone git github.com/coreos/go-systemd v4
clone git github.com/godbus/dbus v4.0.0
clone git github.com/syndtr/gocapability 2c00daeb6c3b45114c80ac44119e7b8801fdd852
clone git github.com/golang/protobuf 1f49d83d9aa00e6ce4fc8258c71cc7786aec968a

# gelf logging driver deps
clone git github.com/Graylog2/go-gelf aab2f594e4585d43468ac57287b0dece9d806883

clone git github.com/fluent/fluent-logger-golang v1.2.0
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
clone git github.com/docker/containerd 837e8c5e1cad013ed57f5c2090c8591c10cbbdae

# cluster
clone git github.com/docker/swarmkit 7e63bdefb94e5bea2641e8bdebae2cfa61a0ed44
clone git github.com/golang/mock bd3c8e81be01eef76d4b503f5e687d2d1354d2d9
clone git github.com/gogo/protobuf v0.3
clone git github.com/cloudflare/cfssl 7fb22c8cba7ecaf98e4082d22d65800cf45e042a
clone git github.com/google/certificate-transparency 0f6e3d1d1ba4d03fdaab7cd716f36255c2e48341
clone git golang.org/x/crypto 3fbbcd23f1cb824e69491a5930cfeff09b12f4d2 https://github.com/golang/crypto.git
clone git golang.org/x/time a4bde12657593d5e90d0533a3e4fd95e635124cb https://github.com/golang/time.git
clone git github.com/mreiferson/go-httpclient 63fe23f7434723dc904c901043af07931f293c47
clone git github.com/hashicorp/go-memdb 98f52f52d7a476958fa9da671354d270c50661a7
clone git github.com/hashicorp/go-immutable-radix 8e8ed81f8f0bf1bdd829593fdd5c29922c1ea990
clone git github.com/hashicorp/golang-lru a0d98a5f288019575c6d1f4bb1573fef2d1fcdc4
clone git github.com/coreos/pkg fa29b1d70f0beaddd4c7021607cc3c3be8ce94b8
clone git github.com/pivotal-golang/clock 3fd3c1944c59d9742e1cd333672181cd1a6f9fa0
clone git github.com/prometheus/client_golang 52437c81da6b127a9925d17eb3a382a2e5fd395e
clone git github.com/beorn7/perks 4c0e84591b9aa9e6dcfdf3e020114cd81f89d5f9
clone git github.com/prometheus/client_model fa8ad6fec33561be4280a8f0514318c79d7f6cb6
clone git github.com/prometheus/common ebdfc6da46522d58825777cf1f90490a5b1ef1d8
clone git github.com/prometheus/procfs abf152e5f3e97f2fafac028d2cc06c1feb87ffa5
clone hg bitbucket.org/ww/goautoneg 75cd24fc2f2c2a2088577d12123ddee5f54e0675
clone git github.com/matttproud/golang_protobuf_extensions fc2b8d3a73c4867e51861bbdd5ae3c1f0869dd6a
clone git github.com/pkg/errors 01fa4104b9c248c8945d14d9f128454d5b28d595

# cli
clone git github.com/spf13/cobra v1.4.1 https://github.com/dnephin/cobra.git
clone git github.com/spf13/pflag cb88ea77998c3f024757528e3305022ab50b43be
clone git github.com/inconshreveable/mousetrap 76626ae9c91c4f2a10f34cad8ce83ea42c93bb75
clone git github.com/flynn-archive/go-shlex 3f9db97f856818214da2e1057f8ad84803971cff

clean

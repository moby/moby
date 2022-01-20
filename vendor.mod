// 'vendor.mod' enables use of 'go mod vendor' to managed 'vendor/' directory.
// There is no 'go.mod' file, as that would imply opting in for all the rules
// around SemVer, which this repo cannot abide by as it uses CalVer.

module github.com/docker/docker

go 1.17

require (
	cloud.google.com/go v0.81.0
	cloud.google.com/go/logging v1.4.2
	github.com/Graylog2/go-gelf v0.0.0-20191017102106-1550ee647df0
	github.com/Microsoft/go-winio v0.5.1
	github.com/Microsoft/hcsshim v0.8.23
	github.com/RackSec/srslog v0.0.0-20180709174129-a4725f04ec91
	github.com/armon/go-radix v0.0.0-20180808171621-7fddfc383310
	github.com/aws/aws-sdk-go v1.28.11
	github.com/bsphere/le_go v0.0.0-20170215134836-7a984a84b549
	github.com/cloudflare/cfssl v0.0.0-20180323000720-5d63dbd981b5
	github.com/containerd/cgroups v1.0.1
	github.com/containerd/containerd v1.5.8
	github.com/containerd/continuity v0.1.0
	github.com/containerd/fifo v1.0.0
	github.com/containerd/typeurl v1.0.2
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/creack/pty v1.1.11
	github.com/deckarep/golang-set v0.0.0-20141123011944-ef32fa3046d9
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker/autogen/winresources/dockerd v0.0.0-00010101000000-000000000000
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c
	github.com/docker/go-metrics v0.0.1
	github.com/docker/go-units v0.4.0
	github.com/docker/libkv v0.2.2-0.20211217103745-e480589147e3
	github.com/docker/libtrust v0.0.0-20150526203908-9cbd2a1374f4
	github.com/docker/swarmkit v1.12.1-0.20210726173615-3629f50980f6
	github.com/fluent/fluent-logger-golang v1.9.0
	github.com/fsnotify/fsnotify v1.4.9
	github.com/godbus/dbus/v5 v5.0.4
	github.com/gogo/protobuf v1.3.2
	github.com/golang/gddo v0.0.0-20190904175337-72a348e765d2
	github.com/google/go-cmp v0.5.5
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/hashicorp/go-immutable-radix v1.0.0
	github.com/hashicorp/go-memdb v0.0.0-20161216180745-cb9a474f84cc
	github.com/hashicorp/memberlist v0.2.4
	github.com/hashicorp/serf v0.8.2
	github.com/imdario/mergo v0.3.12
	github.com/ishidawataru/sctp v0.0.0-20210226210310-f2269e66cdee
	github.com/klauspost/compress v1.12.3
	github.com/miekg/dns v1.1.27
	github.com/mistifyio/go-zfs v2.1.2-0.20190413222219-f784269be439+incompatible
	github.com/moby/buildkit v0.8.2-0.20210615162540-9f254e18360a
	github.com/moby/ipvs v1.0.1
	github.com/moby/locker v1.0.1
	github.com/moby/sys/mount v0.3.0
	github.com/moby/sys/mountinfo v0.5.0
	github.com/moby/sys/signal v0.6.0
	github.com/moby/sys/symlink v0.2.0
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6
	github.com/morikuni/aec v1.0.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.2
	github.com/opencontainers/runc v1.0.3
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417
	github.com/opencontainers/selinux v1.9.1
	github.com/pelletier/go-toml v1.9.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/tchap/go-patricia v2.3.0+incompatible
	github.com/tonistiigi/fsutil v0.0.0-20210609172227-d72af97c0eaf
	github.com/vbatts/tar-split v0.11.2
	github.com/vishvananda/netlink v1.1.1-0.20201029203352-d40f9887b852
	github.com/vishvananda/netns v0.0.0-20200728191858-db3c7e526aae
	go.etcd.io/bbolt v1.3.6
	golang.org/x/net v0.0.0-20211112202133-69e39bad7dc2
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20211025201205-69cdffdb9359
	golang.org/x/time v0.0.0-20211116232009-f0f3c7e86c11
	google.golang.org/genproto v0.0.0-20211208223120-3a66f561d7aa
	google.golang.org/grpc v1.40.0
	gotest.tools/v3 v3.0.3
)

require (
	code.cloudfoundry.org/clock v1.0.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/armon/go-metrics v0.0.0-20180917152333-f0300d1749da // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/cilium/ebpf v0.6.2 // indirect
	github.com/containerd/console v1.0.2 // indirect
	github.com/containerd/go-runc v1.0.0 // indirect
	github.com/containerd/ttrpc v1.1.0 // indirect
	github.com/coreos/etcd v3.3.27+incompatible // indirect
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/cyphar/filepath-securejoin v0.2.2 // indirect
	github.com/fernet/fernet-go v0.0.0-20180830025343-9eac43b88a5e // indirect
	github.com/gofrs/flock v0.7.3 // indirect
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/certificate-transparency-go v1.0.20 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/googleapis/gax-go/v2 v2.0.5 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-msgpack v0.5.3 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jmespath/go-jmespath v0.3.0 // indirect
	github.com/jstemmer/go-junit-report v0.9.1 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/mitchellh/hashstructure v1.0.0 // indirect
	github.com/opentracing-contrib/go-stdlib v1.0.0 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/phayes/permbits v0.0.0-20190612203442-39d7c581d2ee // indirect
	github.com/philhofer/fwd v1.0.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.10.0 // indirect
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/sean-/seed v0.0.0-20170313163322-e2103e2c3529 // indirect
	github.com/tinylib/msgp v1.1.0 // indirect
	github.com/tonistiigi/units v0.0.0-20180711220420-6950e57a87ea // indirect
	github.com/vmihailenco/msgpack v4.0.4+incompatible // indirect
	go.opencensus.io v0.23.0 // indirect
	golang.org/x/crypto v0.0.0-20211202192323-5770296d904e // indirect
	golang.org/x/lint v0.0.0-20201208152925-83fdc39ff7b5 // indirect
	golang.org/x/mod v0.4.1 // indirect
	golang.org/x/oauth2 v0.0.0-20210514164344-f6687ab2804c // indirect
	golang.org/x/text v0.3.6 // indirect
	golang.org/x/tools v0.1.0 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/api v0.46.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	labix.org/v2/mgo v0.0.0-20140701140051-000000000287 // indirect
)

replace (
	cloud.google.com/go => cloud.google.com/go v0.59.0
	cloud.google.com/go/bigquery => cloud.google.com/go/bigquery v1.0.0
	cloud.google.com/go/logging => cloud.google.com/go/logging v1.0.1-0.20190813144457-ceeb313ad77b
	github.com/armon/go-metrics => github.com/armon/go-metrics v0.0.0-20150106224455-eb0af217e5e9
	github.com/armon/go-radix => github.com/armon/go-radix v0.0.0-20150105235045-e39d623f12e8
	github.com/containerd/containerd => github.com/containerd/containerd v1.5.8
	github.com/coreos/go-systemd => github.com/coreos/go-systemd v0.0.0-20180511133405-39ca1b05acc7
	github.com/coreos/pkg => github.com/coreos/pkg v0.0.0-20180108230652-97fdf19511ea
	github.com/docker/distribution => github.com/docker/distribution v2.7.1-0.20190205005809-0d3efadf0154+incompatible
	github.com/gogo/googleapis => github.com/gogo/googleapis v1.3.2
	github.com/golang/protobuf => github.com/golang/protobuf v1.3.5
	github.com/google/go-cmp => github.com/google/go-cmp v0.2.0
	github.com/google/uuid => github.com/google/uuid v1.1.1
	github.com/grpc-ecosystem/go-grpc-middleware => github.com/grpc-ecosystem/go-grpc-middleware v1.2.0
	github.com/hashicorp/go-immutable-radix => github.com/tonistiigi/go-immutable-radix v0.0.0-20170803185627-826af9ccf0fe
	github.com/hashicorp/go-msgpack => github.com/hashicorp/go-msgpack v0.0.0-20140221154404-71c2886f5a67
	github.com/hashicorp/go-multierror => github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/serf => github.com/hashicorp/serf v0.7.1-0.20160317193612-598c54895cc5
	github.com/hpcloud/tail => github.com/hpcloud/tail v1.0.0
	github.com/klauspost/compress => github.com/klauspost/compress v1.11.13
	github.com/matttproud/golang_protobuf_extensions => github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/onsi/ginkgo => github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega => github.com/onsi/gomega v1.5.0
	github.com/pelletier/go-toml => github.com/pelletier/go-toml v1.8.1
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.6.0
	github.com/prometheus/common => github.com/prometheus/common v0.9.1
	github.com/prometheus/procfs => github.com/prometheus/procfs v0.0.11
	github.com/stretchr/testify => github.com/stretchr/testify v1.3.0
	github.com/vishvananda/netlink => github.com/vishvananda/netlink v1.1.0
	go.etcd.io/bbolt => go.etcd.io/bbolt v1.3.5
	go.opencensus.io => go.opencensus.io v0.22.3
	golang.org/x/net => golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
	golang.org/x/oauth2 => golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/text => golang.org/x/text v0.3.3
	golang.org/x/time => golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	google.golang.org/api => google.golang.org/api v0.8.0
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20200227132054-3f1135a288c9
	google.golang.org/grpc => google.golang.org/grpc v1.27.1
	gopkg.in/fsnotify.v1 => gopkg.in/fsnotify.v1 v1.4.7
)

// autogen/winresources/dockerd is generated a build time, this replacement is only for the purpose of `go mod vendor`
replace github.com/docker/docker/autogen/winresources/dockerd => ./hack/make/.resources-windows

// 'vendor.mod' enables use of 'go mod vendor' to managed 'vendor/' directory.
// There is no 'go.mod' file, as that would imply opting in for all the rules
// around SemVer, which this repo cannot abide by as it uses CalVer.

module github.com/docker/docker

go 1.18

require (
	cloud.google.com/go/compute v1.7.0
	cloud.google.com/go/logging v1.4.2
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1
	github.com/Graylog2/go-gelf v0.0.0-20191017102106-1550ee647df0
	github.com/Microsoft/go-winio v0.5.2
	github.com/Microsoft/hcsshim v0.9.6
	github.com/RackSec/srslog v0.0.0-20180709174129-a4725f04ec91
	github.com/armon/go-radix v1.0.1-0.20221118154546-54df44f2176c
	github.com/aws/aws-sdk-go v1.37.0
	github.com/bsphere/le_go v0.0.0-20200109081728-fc06dab2caa8
	github.com/cloudflare/cfssl v0.0.0-20180323000720-5d63dbd981b5
	github.com/containerd/cgroups v1.0.4
	github.com/containerd/containerd v1.6.15
	github.com/containerd/continuity v0.3.0
	github.com/containerd/fifo v1.0.0
	github.com/containerd/typeurl v1.0.2
	github.com/coreos/go-systemd/v22 v22.4.0
	github.com/creack/pty v1.1.11
	github.com/deckarep/golang-set v0.0.0-20141123011944-ef32fa3046d9
	github.com/docker/distribution v2.8.1+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c
	github.com/docker/go-metrics v0.0.1
	github.com/docker/go-units v0.5.0
	github.com/docker/libkv v0.2.2-0.20211217103745-e480589147e3
	github.com/docker/libtrust v0.0.0-20150526203908-9cbd2a1374f4
	github.com/fluent/fluent-logger-golang v1.9.0
	github.com/godbus/dbus/v5 v5.0.6
	github.com/gogo/protobuf v1.3.2
	github.com/golang/gddo v0.0.0-20190904175337-72a348e765d2
	github.com/google/go-cmp v0.5.9
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/hashicorp/go-immutable-radix v1.3.1
	github.com/hashicorp/go-memdb v1.3.2
	github.com/hashicorp/memberlist v0.4.0
	github.com/hashicorp/serf v0.8.5
	github.com/imdario/mergo v0.3.12
	github.com/ishidawataru/sctp v0.0.0-20210707070123-9a39160e9062
	github.com/klauspost/compress v1.15.12
	github.com/miekg/dns v1.1.43
	github.com/mistifyio/go-zfs v2.1.2-0.20190413222219-f784269be439+incompatible
	github.com/moby/buildkit v0.10.6
	github.com/moby/ipvs v1.1.0
	github.com/moby/locker v1.0.1
	github.com/moby/patternmatcher v0.5.0
	github.com/moby/pubsub v1.0.0
	github.com/moby/swarmkit/v2 v2.0.0-20221215132206-0da442b2780f
	github.com/moby/sys/mount v0.3.3
	github.com/moby/sys/mountinfo v0.6.2
	github.com/moby/sys/sequential v0.5.0
	github.com/moby/sys/signal v0.7.0
	github.com/moby/sys/symlink v0.2.0
	github.com/moby/term v0.0.0-20221120202655-abb19827d345
	github.com/morikuni/aec v1.0.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.3-0.20220303224323-02efb9a75ee1
	github.com/opencontainers/runc v1.1.3
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417
	github.com/opencontainers/selinux v1.10.2
	github.com/pelletier/go-toml v1.9.4
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.14.0
	github.com/rootless-containers/rootlesskit v1.1.0
	github.com/sirupsen/logrus v1.9.0
	github.com/spf13/cobra v1.6.1
	github.com/spf13/pflag v1.0.5
	github.com/tchap/go-patricia v2.3.0+incompatible
	github.com/tonistiigi/fsutil v0.0.0-20220315205639-9ed612626da3
	github.com/tonistiigi/go-archvariant v1.0.0
	github.com/vbatts/tar-split v0.11.2
	github.com/vishvananda/netlink v1.2.1-beta.2
	github.com/vishvananda/netns v0.0.2
	go.etcd.io/bbolt v1.3.6
	golang.org/x/net v0.5.0
	golang.org/x/sync v0.1.0
	golang.org/x/sys v0.4.0
	golang.org/x/text v0.6.0
	golang.org/x/time v0.1.0
	google.golang.org/genproto v0.0.0-20220706185917-7780775163c4
	google.golang.org/grpc v1.48.0
	gotest.tools/v3 v3.4.0
)

require (
	cloud.google.com/go v0.102.1 // indirect
	code.cloudfoundry.org/clock v1.0.0 // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/armon/circbuf v0.0.0-20190214190532-5111143e8da2 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/cilium/ebpf v0.7.0 // indirect
	github.com/container-storage-interface/spec v1.5.0 // indirect
	github.com/containerd/console v1.0.3 // indirect
	github.com/containerd/go-runc v1.0.0 // indirect
	github.com/containerd/stargz-snapshotter v0.11.3 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.11.3 // indirect
	github.com/containerd/ttrpc v1.1.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.3 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/felixge/httpsnoop v1.0.2 // indirect
	github.com/fernet/fernet-go v0.0.0-20211208181803-9f70042a33ee // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gofrs/flock v0.8.1 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/certificate-transparency-go v1.1.4 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.1.0 // indirect
	github.com/googleapis/gax-go/v2 v2.4.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.2 // indirect
	github.com/philhofer/fwd v1.1.2 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/sean-/seed v0.0.0-20170313163322-e2103e2c3529 // indirect
	github.com/tinylib/msgp v1.1.6 // indirect
	github.com/tonistiigi/units v0.0.0-20180711220420-6950e57a87ea // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.6 // indirect
	go.etcd.io/etcd/pkg/v3 v3.5.6 // indirect
	go.etcd.io/etcd/raft/v3 v3.5.6 // indirect
	go.etcd.io/etcd/server/v3 v3.5.6 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.29.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace v0.29.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.29.0 // indirect
	go.opentelemetry.io/otel v1.4.1 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.4.1 // indirect
	go.opentelemetry.io/otel/internal/metric v0.27.0 // indirect
	go.opentelemetry.io/otel/metric v0.27.0 // indirect
	go.opentelemetry.io/otel/sdk v1.4.1 // indirect
	go.opentelemetry.io/otel/trace v1.4.1 // indirect
	go.opentelemetry.io/proto/otlp v0.12.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	go.uber.org/zap v1.21.0 // indirect
	golang.org/x/crypto v0.2.0 // indirect
	golang.org/x/oauth2 v0.1.0 // indirect
	google.golang.org/api v0.93.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	k8s.io/klog/v2 v2.80.1 // indirect
)

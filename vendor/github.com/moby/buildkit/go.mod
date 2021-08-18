module github.com/moby/buildkit

go 1.17

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Microsoft/go-winio v0.4.17
	github.com/Microsoft/hcsshim v0.8.18
	github.com/agext/levenshtein v1.2.3
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bits-and-blooms/bitset v1.2.0 // indirect
	github.com/cenkalti/backoff/v4 v4.1.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/containerd/cgroups v1.0.1 // indirect
	github.com/containerd/console v1.0.2
	github.com/containerd/containerd v1.5.5
	github.com/containerd/continuity v0.1.0
	github.com/containerd/fifo v1.0.0 // indirect
	github.com/containerd/fuse-overlayfs-snapshotter v1.0.2
	github.com/containerd/go-cni v1.0.2
	github.com/containerd/go-runc v1.0.0
	github.com/containerd/stargz-snapshotter v0.6.4
	github.com/containerd/stargz-snapshotter/estargz v0.6.4 // indirect
	github.com/containerd/ttrpc v1.0.2 // indirect
	github.com/containerd/typeurl v1.0.2
	github.com/containernetworking/cni v0.8.1 // indirect
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/docker/cli v20.10.7+incompatible
	github.com/docker/distribution v2.7.1+incompatible
	// docker: the actual version is replaced in replace()
	github.com/docker/docker v20.10.7+incompatible // master (v21.xx-dev)
	github.com/docker/docker-credential-helpers v0.6.3 // indirect
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/felixge/httpsnoop v1.0.2 // indirect
	github.com/gofrs/flock v0.7.3
	github.com/gogo/googleapis v1.4.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/protobuf v1.5.2
	// snappy: updated for go1.17 support
	github.com/golang/snappy v0.0.4-0.20210608040537-544b4180ac70 // indirect
	github.com/google/go-cmp v0.5.6
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/google/uuid v1.2.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/hanwen/go-fuse/v2 v2.1.0 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/golang-lru v0.5.3
	github.com/ishidawataru/sctp v0.0.0-20210226210310-f2269e66cdee // indirect
	github.com/klauspost/compress v1.12.3 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/mitchellh/hashstructure v1.0.0
	github.com/moby/locker v1.0.1
	github.com/moby/sys/mount v0.2.0 // indirect
	github.com/moby/sys/mountinfo v0.4.1 // indirect
	github.com/moby/sys/signal v0.5.0
	github.com/moby/term v0.0.0-20201110203204-bea5bbe245bf // indirect
	github.com/morikuni/aec v1.0.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v1.0.1
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417
	github.com/opencontainers/selinux v1.8.2
	github.com/pelletier/go-toml v1.9.3
	github.com/pkg/errors v0.9.1
	github.com/pkg/profile v1.5.0
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.7.1 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.10.0 // indirect
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/russross/blackfriday/v2 v2.0.1 // indirect
	github.com/serialx/hashring v0.0.0-20190422032157-8b2912629002
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/tonistiigi/fsutil v0.0.0-20210818161904-4442383b5028
	github.com/tonistiigi/go-actions-cache v0.0.0-20210714033416-b93d7f1b2e70
	github.com/tonistiigi/units v0.0.0-20180711220420-6950e57a87ea
	github.com/tonistiigi/vt100 v0.0.0-20210615222946-8066bb97264f
	github.com/urfave/cli v1.22.2
	go.etcd.io/bbolt v1.3.5
	go.opencensus.io v0.22.3 // indirect
	go.opentelemetry.io/contrib v0.21.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.21.0
	go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace v0.21.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.21.0
	go.opentelemetry.io/otel v1.0.0-RC1
	go.opentelemetry.io/otel/exporters/jaeger v1.0.0-RC1
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.0.0-RC1
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.0.0-RC1
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.0.0-RC1
	go.opentelemetry.io/otel/internal/metric v0.21.0 // indirect
	go.opentelemetry.io/otel/metric v0.21.0 // indirect
	go.opentelemetry.io/otel/sdk v1.0.0-RC1
	go.opentelemetry.io/otel/trace v1.0.0-RC1
	go.opentelemetry.io/proto/otlp v0.9.0
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c
	golang.org/x/text v0.3.4 // indirect
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	google.golang.org/genproto v0.0.0-20201110150050-8816d57aaa9a
	google.golang.org/grpc v1.38.0
	google.golang.org/protobuf v1.26.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c // indirect
	gotest.tools/v3 v3.0.3 // indirect
)

replace (
	github.com/docker/docker => github.com/docker/docker v20.10.3-0.20210817025855-ba2adeebdb8d+incompatible
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc => github.com/tonistiigi/opentelemetry-go-contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.0.0-20210714055410-d010b05b4939
	go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace => github.com/tonistiigi/opentelemetry-go-contrib/instrumentation/net/http/httptrace/otelhttptrace v0.0.0-20210714055410-d010b05b4939
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp => github.com/tonistiigi/opentelemetry-go-contrib/instrumentation/net/http/otelhttp v0.0.0-20210714055410-d010b05b4939
)

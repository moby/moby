module github.com/containerd/containerd

go 1.16

require (
	github.com/Microsoft/go-winio v0.4.17
	github.com/Microsoft/hcsshim v0.8.18
	github.com/containerd/aufs v1.0.0
	github.com/containerd/btrfs v1.0.0
	github.com/containerd/cgroups v1.0.1
	github.com/containerd/console v1.0.2
	github.com/containerd/continuity v0.1.0
	github.com/containerd/fifo v1.0.0
	github.com/containerd/go-cni v1.0.2
	github.com/containerd/go-runc v1.0.0
	github.com/containerd/imgcrypt v1.1.1
	github.com/containerd/nri v0.1.0
	github.com/containerd/ttrpc v1.0.2
	github.com/containerd/typeurl v1.0.2
	github.com/containerd/zfs v1.0.0
	github.com/containernetworking/plugins v0.9.1
	github.com/coreos/go-systemd/v22 v22.1.0
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c
	github.com/docker/go-metrics v0.0.1
	github.com/docker/go-units v0.4.0
	github.com/emicklei/go-restful v2.9.5+incompatible
	github.com/fsnotify/fsnotify v1.4.9
	github.com/gogo/googleapis v1.4.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.4.3
	github.com/google/go-cmp v0.5.4
	github.com/google/uuid v1.2.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/hashicorp/go-multierror v1.0.0
	github.com/imdario/mergo v0.3.11
	github.com/klauspost/compress v1.11.13
	github.com/moby/locker v1.0.1
	github.com/moby/sys/mountinfo v0.4.1
	github.com/moby/sys/symlink v0.1.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v1.0.0-rc93
	github.com/opencontainers/runtime-spec v1.0.3-0.20200929063507-e6143ca7d51d
	github.com/opencontainers/selinux v1.8.0
	github.com/pelletier/go-toml v1.8.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/procfs v0.6.0 // indirect; temporarily force v0.6.0, which was previously defined in imgcrypt as explicit version
	github.com/satori/go.uuid v1.2.0 // indirect
	github.com/sirupsen/logrus v1.7.0
	github.com/stretchr/testify v1.6.1
	github.com/tchap/go-patricia v2.2.6+incompatible
	github.com/urfave/cli v1.22.2
	go.etcd.io/bbolt v1.3.5
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a
	golang.org/x/sys v0.0.0-20210324051608-47abb6519492
	google.golang.org/grpc v1.33.2
	gotest.tools/v3 v3.0.3
	k8s.io/api v0.20.6
	k8s.io/apimachinery v0.20.6
	k8s.io/apiserver v0.20.6
	k8s.io/client-go v0.20.6
	k8s.io/component-base v0.20.6
	k8s.io/cri-api v0.20.6
	k8s.io/klog/v2 v2.4.0
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
)

// When updating replace rules, make sure to also update the rules in integration/client/go.mod
replace (
	// prevent transitional dependencies due to containerd having a circular
	// dependency on itself through plugins. see .empty-mod/go.mod for details
	github.com/containerd/containerd => ./.empty-mod/
	github.com/gogo/googleapis => github.com/gogo/googleapis v1.3.2
	github.com/golang/protobuf => github.com/golang/protobuf v1.3.5
	// urfave/cli must be <= v1.22.1 due to a regression: https://github.com/urfave/cli/issues/1092
	github.com/urfave/cli => github.com/urfave/cli v1.22.1
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20200224152610-e50cd9704f63
	google.golang.org/grpc => google.golang.org/grpc v1.27.1
)

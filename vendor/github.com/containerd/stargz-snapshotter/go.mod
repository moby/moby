module github.com/containerd/stargz-snapshotter

go 1.16

require (
	github.com/containerd/console v1.0.3
	github.com/containerd/containerd v1.5.5
	github.com/containerd/continuity v0.1.0
	github.com/containerd/go-cni v1.0.2
	github.com/containerd/stargz-snapshotter/estargz v0.8.0
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/docker/cli v20.10.8+incompatible
	github.com/docker/docker v20.10.7+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.6.4 // indirect
	github.com/docker/go-metrics v0.0.1
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da
	github.com/hanwen/go-fuse/v2 v2.1.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/klauspost/compress v1.13.5
	github.com/moby/sys/mountinfo v0.4.1
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.2-0.20210819154149-5ad6f50d6283
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417
	github.com/pelletier/go-toml v1.9.4
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/rs/xid v1.3.0
	github.com/sirupsen/logrus v1.8.1
	github.com/urfave/cli v1.22.4
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210616094352-59db8d763f22
	google.golang.org/grpc v1.40.0
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/cri-api v0.22.1
)

replace (
	// Import local package for estargz.
	github.com/containerd/stargz-snapshotter/estargz => ./estargz

	// NOTE1: github.com/containerd/containerd v1.4.0 depends on github.com/urfave/cli v1.22.1
	//        because of https://github.com/urfave/cli/issues/1092
	// NOTE2: Automatic upgrade of this is disabled in denendabot.yml. When we remove this replace
	//        directive, we must remove the corresponding "ignore" configuration from dependabot.yml
	github.com/urfave/cli => github.com/urfave/cli v1.22.1
)

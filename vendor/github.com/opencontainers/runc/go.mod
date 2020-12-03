module github.com/opencontainers/runc

go 1.14

require (
	github.com/checkpoint-restore/go-criu/v4 v4.1.0
	github.com/cilium/ebpf v0.2.0
	github.com/containerd/console v1.0.1
	github.com/coreos/go-systemd/v22 v22.1.0
	github.com/cyphar/filepath-securejoin v0.2.2
	github.com/docker/go-units v0.4.0
	github.com/godbus/dbus/v5 v5.0.3
	github.com/golang/protobuf v1.4.3
	github.com/moby/sys/mountinfo v0.4.0
	github.com/mrunalp/fileutils v0.5.0
	github.com/opencontainers/runtime-spec v1.0.3-0.20200929063507-e6143ca7d51d
	github.com/opencontainers/selinux v1.6.0
	github.com/pkg/errors v0.9.1
	github.com/seccomp/libseccomp-golang v0.9.1
	github.com/sirupsen/logrus v1.7.0
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	// NOTE: urfave/cli must be <= v1.22.1 due to a regression: https://github.com/urfave/cli/issues/1092
	github.com/urfave/cli v1.22.1
	github.com/vishvananda/netlink v1.1.0
	github.com/willf/bitset v1.1.11
	golang.org/x/sys v0.0.0-20201107080550-4d91cf3a1aaf
)

module github.com/moby/buildkit

go 1.12

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/Microsoft/go-winio v0.4.13-0.20190408173621-84b4ab48a507
	github.com/Microsoft/hcsshim v0.8.5 // indirect
	github.com/apache/thrift v0.0.0-20161221203622-b2a4d4ae21c7 // indirect
	github.com/codahale/hdrhistogram v0.0.0-20160425231609-f8ad88b59a58 // indirect
	github.com/containerd/cgroups v0.0.0-20190226200435-dbea6f2bd416 // indirect
	github.com/containerd/console v0.0.0-20181022165439-0650fd9eeb50
	github.com/containerd/containerd v1.3.0-0.20190507210959-7c1e88399ec0
	github.com/containerd/continuity v0.0.0-20190827140505-75bee3e2ccb6
	github.com/containerd/fifo v0.0.0-20180307165137-3d5202aec260 // indirect
	github.com/containerd/go-cni v0.0.0-20190610170741-5a4663dad645
	github.com/containerd/go-runc v0.0.0-20190911050354-e029b79d8cda
	github.com/containerd/ttrpc v0.0.0-20190411181408-699c4e40d1e7 // indirect
	github.com/containerd/typeurl v0.0.0-20180627222232-a93fcdb778cd // indirect
	github.com/containernetworking/cni v0.6.1-0.20180218032124-142cde0c766c // indirect
	github.com/coreos/go-systemd v0.0.0-20190321100706-95778dfbb74e
	github.com/docker/cli v0.0.0-20190321234815-f40f9c240ab0
	github.com/docker/distribution v2.7.1-0.20190205005809-0d3efadf0154+incompatible
	github.com/docker/docker v1.14.0-0.20190319215453-e7b5f7dbe98c
	github.com/docker/docker-credential-helpers v0.6.0 // indirect
	github.com/docker/go-connections v0.3.0
	github.com/docker/go-events v0.0.0-20170721190031-9461782956ad // indirect
	github.com/docker/libnetwork v0.8.0-dev.2.0.20190604151032-3c26b4e7495e
	github.com/godbus/dbus v4.1.0+incompatible // indirect
	github.com/gofrs/flock v0.7.0
	github.com/gogo/googleapis v1.1.0
	github.com/gogo/protobuf v1.2.0
	github.com/golang/protobuf v1.2.0
	github.com/google/go-cmp v0.2.0
	github.com/google/shlex v0.0.0-20150127133951-6f45313302b9
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645
	github.com/hashicorp/go-immutable-radix v1.0.0
	github.com/hashicorp/golang-lru v0.0.0-20160207214719-a0d98a5f2880
	github.com/hashicorp/uuid v0.0.0-20160311170451-ebb0a03e909c // indirect
	github.com/ishidawataru/sctp v0.0.0-20180213033435-07191f837fed // indirect
	github.com/jaguilar/vt100 v0.0.0-20150826170717-2703a27b14ea
	github.com/kr/pretty v0.1.0 // indirect
	github.com/mitchellh/hashstructure v0.0.0-20170609045927-2bca23e0e452
	github.com/morikuni/aec v0.0.0-20170113033406-39771216ff4c
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v1.0.0-rc8
	github.com/opencontainers/runtime-spec v0.0.0-20180909173843-eba862dc2470
	github.com/opencontainers/selinux v1.0.0-rc1.0.20180628160156-b6fa367ed7f5
	github.com/opentracing-contrib/go-stdlib v0.0.0-20171029140428-b1a47cfbdd75
	github.com/opentracing/opentracing-go v0.0.0-20171003133519-1361b9cd60be
	github.com/pkg/errors v0.8.1
	github.com/pkg/profile v1.2.1
	github.com/serialx/hashring v0.0.0-20190422032157-8b2912629002
	github.com/sirupsen/logrus v1.3.0
	github.com/stretchr/testify v1.3.0
	github.com/syndtr/gocapability v0.0.0-20180916011248-d98352740cb2 // indirect
	github.com/tonistiigi/fsutil v0.0.0-20200128191323-6c909ab392c1
	github.com/tonistiigi/units v0.0.0-20180711220420-6950e57a87ea
	github.com/uber/jaeger-client-go v0.0.0-20180103221425-e02c85f9069e
	github.com/uber/jaeger-lib v1.2.1 // indirect
	github.com/urfave/cli v0.0.0-20171014202726-7bc6a0acffa5
	github.com/vishvananda/netlink v1.0.0 // indirect
	github.com/vishvananda/netns v0.0.0-20180720170159-13995c7128cc // indirect
	go.etcd.io/bbolt v1.3.2
	golang.org/x/crypto v0.0.0-20190308221718-c2843e01d9a2
	golang.org/x/net v0.0.0-20190311183353-d8887717615a
	golang.org/x/sync v0.0.0-20180314180146-1d60e4601c6f
	golang.org/x/sys v0.0.0-20190303122642-d455e41777fc
	golang.org/x/time v0.0.0-20161028155119-f51c12702a4d
	google.golang.org/genproto v0.0.0-20180817151627-c66870c02cf8
	google.golang.org/grpc v1.20.1
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gotest.tools v2.2.0+incompatible
)

replace github.com/hashicorp/go-immutable-radix => github.com/tonistiigi/go-immutable-radix v0.0.0-20170803185627-826af9ccf0fe

replace github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305

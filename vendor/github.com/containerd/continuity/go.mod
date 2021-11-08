module github.com/containerd/continuity

go 1.13

require (
	// 5883e5a4b512fe2e32f915b1c66a1ddfef81cb3f is the last version to support macOS
	// see https://github.com/bazil/fuse/commit/60eaf8f021ce00e5c52529cdcba1067e13c1c2c2
	bazil.org/fuse v0.0.0-20200407214033-5883e5a4b512
	github.com/dustin/go-humanize v1.0.0
	github.com/golang/protobuf v1.3.5
	github.com/opencontainers/go-digest v1.0.0
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/cobra v1.0.0
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a
	golang.org/x/sys v0.0.0-20210124154548-22da62e12c0c
)

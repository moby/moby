module github.com/containerd/containerd/api

go 1.16

require (
	github.com/containerd/ttrpc v1.0.2
	github.com/containerd/typeurl v1.0.2
	github.com/gogo/googleapis v1.4.0
	github.com/gogo/protobuf v1.3.2
	github.com/opencontainers/go-digest v1.0.0
	google.golang.org/grpc v1.41.0
)

replace (
	github.com/gogo/googleapis => github.com/gogo/googleapis v1.3.2
	// urfave/cli must be <= v1.22.1 due to a regression: https://github.com/urfave/cli/issues/1092
	github.com/urfave/cli => github.com/urfave/cli v1.22.1
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20200224152610-e50cd9704f63
)

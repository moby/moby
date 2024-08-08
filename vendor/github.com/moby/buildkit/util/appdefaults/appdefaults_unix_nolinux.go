//go:build unix && !linux

package appdefaults

const (
	Address         = "unix:///var/run/buildkit/buildkitd.sock"
	traceSocketPath = "/var/run/buildkit/otel-grpc.sock"
)

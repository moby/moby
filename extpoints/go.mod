module github.com/moby/moby/extpoints

go 1.26.0

require (
	github.com/containerd/errdefs v1.0.0
	github.com/containerd/errdefs/pkg v0.3.0
	github.com/moby/extensions v0.0.0-20261120190433-fb779cf107cd
	google.golang.org/grpc v1.85.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/containerd/log v0.2.0 // indirect
	github.com/containerd/typeurl/v2 v2.3.0 // indirect
	github.com/sirupsen/logrus v1.10.2 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261012220534-4341edf3f941 // indirect
)

tool github.com/moby/extensions/cmd/mobyextgen

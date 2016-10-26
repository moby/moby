package plugin

//go:generate protoc -I.:../../vendor/github.com/gogo/protobuf/protobuf --gogoswarm_out=import_path=github.com/docker/swarmkit/protobuf/plugin,Mgoogle/protobuf/descriptor.proto=github.com/gogo/protobuf/protoc-gen-gogo/descriptor:. plugin.proto

//go:generate protoc -I.:../../vendor:../../vendor/github.com/gogo/protobuf --gogoswarm_out=plugins=grpc+deepcopy+raftproxy+authenticatedwrapper,import_path=github.com/docker/swarmkit/api/duration,Mgogoproto/gogo.proto=github.com/gogo/protobuf/gogoproto:. duration.proto

package duration

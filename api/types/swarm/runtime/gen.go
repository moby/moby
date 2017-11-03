//go:generate protoc -I . --gogofast_out=import_path=github.com/moby/moby/api/types/swarm/runtime:. plugin.proto

package runtime

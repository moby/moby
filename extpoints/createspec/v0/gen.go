//go:generate bash -c "cd ../../.. && go run ./internal/extensions/cmd/mobyextgen -dir extpoints/createspec/v0 -import github.com/moby/moby/v2/extpoints/createspec/v0 -proto create_spec_hook.proto && protoc --go_out=. --go_opt=module=github.com/moby/moby/v2 --go-grpc_out=. --go-grpc_opt=module=github.com/moby/moby/v2 -I . extpoints/createspec/v0/create_spec_hook.proto"

// Package createspecv0 is the create-spec hook extension point contract, written
// Go-first: the [Hook] interface and its types in createspec.go are the source
// of truth, and this package stays free of any protobuf/gRPC imports. mobyextgen
// generates the .proto from them and the transport wiring into the protogen
// subpackage, where protoc also generates the proto messages and gRPC code.
// Regenerate with `go generate ./extpoints/createspec/v0/`.
package createspecv0

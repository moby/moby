//go:generate bash -c "cd ../../../.. && protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative -I . internal/extensions/sdk/sdkpb/runtime.proto"

// Package sdkpb holds the generated extension runtime service contract (the
// Describe RPC and the wire form of an extension declaration). Regenerate with
// `go generate ./internal/extensions/sdk/sdkpb/`.
package sdkpb

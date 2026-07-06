//go:generate bash -c "cd ../../../../../.. && go run ./internal/extensions/cmd/mobyextgen -dir internal/extensions/internal/launcher/echo/v1 -import github.com/moby/moby/v2/internal/extensions/internal/launcher/echo/v1 -proto echo.proto && protoc --go_out=. --go_opt=module=github.com/moby/moby/v2 --go-grpc_out=. --go-grpc_opt=module=github.com/moby/moby/v2 -I . internal/extensions/internal/launcher/echo/v1/echo.proto"

// Package echov1 is a minimal extension point used only by the launcher
// end-to-end test, to exercise the launcher against a point that is not any
// real one. It is written Go-first like the real points: the [EchoServer]
// interface and its messages in echo.go are the source of truth, and mobyextgen
// generates the .proto and transport wiring into the protogen subpackage, where
// protoc also generates the proto messages and gRPC code. Regenerate with
// `go generate ./internal/extensions/internal/launcher/echo/v1/`.
package echov1

//go:generate bash -c "cd ../../../../.. && go run ./internal/extensions/cmd/mobyextgen -dir internal/extensions/example/greeter/v0 -import github.com/moby/moby/v2/internal/extensions/example/greeter/v0 -proto greeter.proto && protoc --go_out=. --go_opt=module=github.com/moby/moby/v2 --go-grpc_out=. --go-grpc_opt=module=github.com/moby/moby/v2 -I . internal/extensions/example/greeter/v0/greeter.proto"

package greeterv0

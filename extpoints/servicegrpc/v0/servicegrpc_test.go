package servicegrpcv0

import (
	"strings"
	"testing"

	"github.com/moby/extensions"
	servicev0 "github.com/moby/extensions/extpoints/service/v0"
	"github.com/moby/extensions/sdk"
	"google.golang.org/grpc"
)

type emptyProvider struct{}

func (emptyProvider) RegisterServices(grpc.ServiceRegistrar) {}

func TestRejectEmptyOffer(t *testing.T) {
	t.Parallel()

	ext := extensions.New(extensions.Declaration{
		ID: "org.example.empty.v1",
		Providers: []extensions.Provider{
			Point.Provide(emptyProvider{}),
			servicev0.Offer(Point),
		},
	})
	err := sdk.NewServer().Register(ext, ServerPoint)
	if err == nil || !strings.Contains(err.Error(), "registered no gRPC service") {
		t.Fatalf("Register error = %v, want it to contain %q", err, "registered no gRPC service")
	}
}

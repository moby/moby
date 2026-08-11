package host_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moby/moby/v2/integration/extension/testdata/greeterdep"
	"github.com/moby/moby/v2/internal/extensions"
	greeterv0 "github.com/moby/moby/v2/internal/extensions/example/greeter/v0"
	greeterpb "github.com/moby/moby/v2/internal/extensions/example/greeter/v0/protogen"
	"github.com/moby/moby/v2/internal/extensions/host"
	"github.com/moby/moby/v2/internal/extensions/serverpoint"
	"gotest.tools/v3/assert"
)

type recordingGreeter struct{ calls *atomic.Int32 }

func (g recordingGreeter) Greet(_ context.Context, req *greeterv0.HelloRequest) (*greeterv0.HelloReply, error) {
	g.calls.Add(1)
	return &greeterv0.HelloReply{Message: "hello " + req.Name}, nil
}

func TestOutOfProcessDependency(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and launches a helper binary")
	}

	dir := t.TempDir()
	bin := filepath.Join(dir, greeterdep.ID)
	build := exec.Command("go", "build", "-o", bin, "github.com/moby/moby/v2/integration/extension/testdata/greeterdep/cmd/greeterdep")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build greeterdep extension: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var calls atomic.Int32
	greeter := extensions.New(extensions.Declaration{
		ID:        "org.mobyproject.example.greeter-provider.v1",
		Providers: []extensions.Provider{greeterv0.Point.Provide(recordingGreeter{calls: &calls})},
	})

	h, err := host.New(ctx, host.Options{
		RuntimeDir:          t.TempDir(),
		Extensions:          []extensions.Extension{greeter},
		Dirs:                []string{dir},
		DependencyProviders: []serverpoint.Registration{greeterpb.ServerPoint},
	})
	assert.NilError(t, err)
	defer func() { assert.NilError(t, h.Shutdown(context.Background())) }()

	assert.Equal(t, calls.Load(), int32(1),
		"in-process greeter provider was not called by the out-of-process extension")
}

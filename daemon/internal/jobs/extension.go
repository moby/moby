package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/moby/extensions"
	jobspb "github.com/moby/moby/v2/extpoints/jobs/api/v0/protogen"
	servicegrpcv0 "github.com/moby/moby/v2/extpoints/servicegrpc/v0"
	"google.golang.org/grpc"
)

// ExtensionID identifies the builtin jobs extension.
const ExtensionID = "org.mobyproject.jobs.v1"

// Extension is the builtin jobs extension: it exposes the Jobs API on the
// daemon socket through the service.grpc point, and bridges the daemon's
// container backend into the jobs manager.
//
// The extension host initializes during daemon construction, before the
// container backend can serve requests, so the manager cannot be built at
// registration time. The daemon activates the extension explicitly once
// container restore has completed; API calls arriving before that are
// answered with Unavailable.
type Extension struct {
	root string

	mu      sync.Mutex
	manager *Manager
}

// NewExtension builds the jobs extension, persisting its state under root.
func NewExtension(root string) *Extension {
	return &Extension{root: root}
}

// Declaration registers the extension with the host.
func (e *Extension) Declaration() extensions.Declaration {
	return extensions.Declaration{
		ID:        ExtensionID,
		Providers: []extensions.Provider{servicegrpcv0.Point.Provide(serviceExposer{ext: e})},
		Shutdown:  e.shutdownManager,
	}
}

// Activate loads the store, starts the scheduler, and opens the API for
// business. The daemon calls it exactly once, when its container backend is
// ready; a second activation is a programming error and is rejected rather
// than silently leaking the previous manager. The mutex is held across the
// whole activation: it is a one-shot startup step, and API calls landing
// during it wait instead of racing it.
func (e *Extension) Activate(ctx context.Context, backend Backend) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.manager != nil {
		return errors.New("the jobs extension is already active")
	}
	// The manager outlives the activation call: its scheduler and watchers
	// must not be tied to the startup context's cancellation.
	ctx = context.WithoutCancel(ctx)
	store, err := NewStore(ctx, e.root)
	if err != nil {
		return fmt.Errorf("activating the jobs extension: %w", err)
	}
	manager := NewManager(store, backend)
	// Order matters: reconciliation resolves runs orphaned by an unclean
	// stop from the containers' actual state, so triggers re-arm from a
	// truthful picture.
	manager.Restore(ctx)
	manager.Start(ctx)
	e.manager = manager
	return nil
}

// shutdownManager stops the active manager; a never-activated extension has
// nothing to stop.
func (e *Extension) shutdownManager(ctx context.Context) error {
	e.mu.Lock()
	manager := e.manager
	e.manager = nil
	e.mu.Unlock()
	if manager == nil {
		return nil
	}
	return manager.Shutdown(ctx)
}

// serviceExposer plugs the Jobs service into the daemon's socket exposure.
type serviceExposer struct {
	ext *Extension
}

// RegisterServices registers the Jobs service on the daemon's gRPC server.
func (p serviceExposer) RegisterServices(r grpc.ServiceRegistrar) {
	jobspb.ServerPoint.Register(r, &service{ext: p.ext})
}

package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/containerd/log"
	"github.com/moby/extensions"
	servicev0 "github.com/moby/extensions/extpoints/service/v0"
	jobsv0 "github.com/moby/moby/v2/extpoints/jobs/api/v0"
	runtimev0 "github.com/moby/moby/v2/extpoints/runtime/v0"
)

// ExtensionID identifies the builtin jobs extension.
const ExtensionID = "org.mobyproject.jobs.v1"

// Extension is the builtin jobs extension. It provides the Jobs point and
// offers it for publication on the daemon socket; the container runtime
// point it depends on supplies the backend that runs job containers.
//
// The extension host initializes during daemon construction, before the
// runtime can serve container operations, so activation waits out the
// runtime's readiness gate in the background. API calls arriving before
// activation completes are answered with Unavailable.
type Extension struct {
	root string

	mu               sync.Mutex
	manager          *Manager
	cancelActivation context.CancelFunc
}

// NewExtension builds the jobs extension, persisting its state under root.
func NewExtension(root string) *Extension {
	return &Extension{root: root}
}

// Declaration describes the extension to the host.
func (e *Extension) Declaration() extensions.Declaration {
	return extensions.Declaration{
		ID: ExtensionID,
		Providers: []extensions.Provider{
			jobsv0.Point.Provide(&service{ext: e}),
			servicev0.Offer(jobsv0.Point),
		},
		Dependencies: []extensions.Dependency{runtimev0.Point.Dependency()},
		Init:         e.init,
		Shutdown:     e.shutdownManager,
	}
}

// init resolves the runtime dependency, builds the manager — an unreadable
// state directory fails daemon startup loudly rather than surfacing later as
// a mysteriously empty API — and finishes activation in the background:
// container operations only work once the runtime reports ready, which
// happens after the host (and this init) runs.
func (e *Extension) init(ctx context.Context, _ extensions.Config, resolver extensions.Resolver) error {
	backend, err := runtimev0.Point.Single(resolver)
	if err != nil {
		return fmt.Errorf("resolving the container runtime point: %w", err)
	}
	manager, err := e.buildManager(ctx, backend)
	if err != nil {
		return err
	}
	// The manager outlives the init call, so its lifetime cannot be tied to
	// the startup context's cancellation; shutdownManager cancels the
	// readiness wait instead, releasing the goroutine if the runtime never
	// becomes ready.
	activateCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	e.mu.Lock()
	e.cancelActivation = cancel
	e.mu.Unlock()
	go func() {
		if err := backend.Ready(activateCtx); err != nil {
			// Cancelled means shutdownManager released the wait: an orderly
			// stop, not a failure worth alarming about.
			if activateCtx.Err() == nil {
				log.G(activateCtx).WithError(err).Error("jobs: the container runtime never became ready; the Jobs API stays unavailable")
			}
			return
		}
		if err := e.activate(activateCtx, manager); err != nil && activateCtx.Err() == nil {
			log.G(activateCtx).WithError(err).Error("jobs: activation failed; the Jobs API stays unavailable")
		}
	}()
	return nil
}

// buildManager loads the store under the extension's root and builds the
// manager driving backend. The store is pure file I/O and needs no running
// container backend.
func (e *Extension) buildManager(ctx context.Context, backend Backend) (*Manager, error) {
	store, err := NewStore(ctx, e.root)
	if err != nil {
		return nil, err
	}
	return NewManager(store, backend), nil
}

// activate reconciles the manager's runs, starts the scheduler, and opens
// the API for business. It runs exactly once, when the runtime is ready; a
// second activation is a programming error and is rejected rather than
// silently leaking the previous manager. The mutex is held across the whole
// activation: it is a one-shot startup step, and API calls landing during it
// wait instead of racing it.
func (e *Extension) activate(ctx context.Context, manager *Manager) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.manager != nil {
		return errors.New("the jobs extension is already active")
	}
	// A shutdown racing activation cancels the context before releasing the
	// mutex: checking under the same mutex means a manager never starts
	// after the extension was shut down.
	if err := ctx.Err(); err != nil {
		return err
	}
	// Order matters: reconciliation resolves runs orphaned by an unclean
	// stop from the containers' actual state, so triggers re-arm from a
	// truthful picture.
	manager.Restore(ctx)
	manager.Start(ctx)
	e.manager = manager
	return nil
}

// shutdownManager stops the active manager; a never-activated extension has
// only its readiness wait to release.
func (e *Extension) shutdownManager(ctx context.Context) error {
	e.mu.Lock()
	if e.cancelActivation != nil {
		e.cancelActivation()
	}
	manager := e.manager
	e.manager = nil
	e.mu.Unlock()
	if manager == nil {
		return nil
	}
	return manager.Shutdown(ctx)
}

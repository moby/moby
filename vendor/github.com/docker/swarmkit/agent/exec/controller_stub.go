package exec

import (
	"github.com/docker/swarmkit/api"
	"golang.org/x/net/context"
	"runtime"
	"strings"
)

// StubController implements the Controller interface,
// but allows you to specify behaviors for each of its methods.
type StubController struct {
	Controller
	UpdateFn    func(ctx context.Context, t *api.Task) error
	PrepareFn   func(ctx context.Context) error
	StartFn     func(ctx context.Context) error
	WaitFn      func(ctx context.Context) error
	ShutdownFn  func(ctx context.Context) error
	TerminateFn func(ctx context.Context) error
	RemoveFn    func(ctx context.Context) error
	CloseFn     func() error
	calls       map[string]int
	cstatus     *api.ContainerStatus
}

// NewStubController returns an initialized StubController
func NewStubController() *StubController {
	return &StubController{
		calls: make(map[string]int),
	}
}

// If function A calls updateCountsForSelf,
// The callCount[A] value will be incremented
func (sc *StubController) called() {
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		panic("Failed to find caller of function")
	}
	// longName looks like 'github.com/docker/swarmkit/agent/exec.(*StubController).Prepare:1'
	longName := runtime.FuncForPC(pc).Name()
	parts := strings.Split(longName, ".")
	tail := strings.Split(parts[len(parts)-1], ":")
	sc.calls[tail[0]]++
}

// Update is part of the Controller interface
func (sc *StubController) Update(ctx context.Context, t *api.Task) error {
	sc.called()
	return sc.UpdateFn(ctx, t)
}

// Prepare is part of the Controller interface
func (sc *StubController) Prepare(ctx context.Context) error { sc.called(); return sc.PrepareFn(ctx) }

// Start is part of the Controller interface
func (sc *StubController) Start(ctx context.Context) error { sc.called(); return sc.StartFn(ctx) }

// Wait is part of the Controller interface
func (sc *StubController) Wait(ctx context.Context) error { sc.called(); return sc.WaitFn(ctx) }

// Shutdown is part of the Controller interface
func (sc *StubController) Shutdown(ctx context.Context) error { sc.called(); return sc.ShutdownFn(ctx) }

// Terminate is part of the Controller interface
func (sc *StubController) Terminate(ctx context.Context) error {
	sc.called()
	return sc.TerminateFn(ctx)
}

// Remove is part of the Controller interface
func (sc *StubController) Remove(ctx context.Context) error { sc.called(); return sc.RemoveFn(ctx) }

// Close is part of the Controller interface
func (sc *StubController) Close() error { sc.called(); return sc.CloseFn() }

package broker

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/moby/extensions"
)

// Broker registers extensions, resolves dependencies, and exposes providers.
// It is safe for concurrent use. User Init and Shutdown callbacks run without
// the broker lock because they may resolve providers.
type Broker struct {
	mu         sync.RWMutex
	extensions map[extensions.ExtensionID]*extensionState
	order      []extensions.ExtensionID
	initOrder  []extensions.ExtensionID
}

type extensionState struct {
	extension   extensions.Declaration
	identity    extensions.ExtensionIdentity
	initialized bool
}

// New creates an empty Broker.
func New() *Broker {
	return &Broker{extensions: make(map[extensions.ExtensionID]*extensionState)}
}

// Register adds an extension under the identity attested by its host.
func (b *Broker) Register(identity extensions.ExtensionIdentity, ext extensions.Extension) error {
	if err := extensions.ValidateExtensionIdentity(identity); err != nil {
		return err
	}
	decl := ext.Declaration()
	if identity.ID != decl.ID {
		return fmt.Errorf("extension identity id %q does not match declared id %q", identity.ID, decl.ID)
	}
	seenPoints := make(map[extensions.PointID]struct{})
	for _, provider := range decl.Providers {
		if provider.Point == "" {
			return fmt.Errorf("extension %q has provider without point", decl.ID)
		}
		if provider.Impl == nil {
			return fmt.Errorf("extension %q provider for point %q is nil", decl.ID, provider.Point)
		}
		if _, ok := seenPoints[provider.Point]; ok {
			return fmt.Errorf("extension %q implements point %q more than once", decl.ID, provider.Point)
		}
		seenPoints[provider.Point] = struct{}{}
	}
	for _, dep := range decl.Dependencies {
		if dep.Point == "" && dep.Extension == "" {
			return fmt.Errorf("extension %q has dependency without point or extension", decl.ID)
		}
		if dep.Point != "" && dep.Extension != "" {
			return fmt.Errorf("extension %q dependency must name either point or extension", decl.ID)
		}
	}
	for _, conflict := range decl.Conflicts {
		if conflict == "" {
			return fmt.Errorf("extension %q has empty conflict id", decl.ID)
		}
		if conflict == decl.ID {
			return fmt.Errorf("extension %q conflicts with itself", decl.ID)
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.extensions[decl.ID]; ok {
		return fmt.Errorf("extension %q is already registered", decl.ID)
	}
	if err := b.checkConflicts(decl); err != nil {
		return err
	}

	b.extensions[decl.ID] = &extensionState{extension: decl, identity: identity}
	b.order = append(b.order, decl.ID)
	return nil
}

func (b *Broker) checkConflicts(ext extensions.Declaration) error {
	for _, existingID := range b.order {
		existing := b.extensions[existingID].extension
		if slices.Contains(ext.Conflicts, existing.ID) || slices.Contains(existing.Conflicts, ext.ID) {
			return fmt.Errorf("extension %q conflicts with extension %q", ext.ID, existing.ID)
		}
	}
	return nil
}

// Init resolves dependencies and initializes all registered extensions in
// order, delivering each its configuration from configs (keyed by extension id).
func (b *Broker) Init(ctx context.Context, configs map[extensions.ExtensionID]extensions.Config) error {
	b.mu.Lock()
	resolved, err := b.resolveOrder()
	if err != nil {
		b.mu.Unlock()
		return err
	}
	// Record the order before initializing so partial initialization can unwind in
	// reverse order.
	b.initOrder = resolved
	b.mu.Unlock()

	for _, id := range resolved {
		b.mu.RLock()
		state := b.extensions[id]
		initialized, initFn := state.initialized, state.extension.Init
		b.mu.RUnlock()
		if initialized {
			continue
		}
		// Init may resolve providers, so do not hold the lock across the callback.
		if initFn != nil {
			if err := initFn(ctx, configs[id], b); err != nil {
				return fmt.Errorf("initialize extension %q: %w", id, err)
			}
		}
		b.mu.Lock()
		state.initialized = true
		b.mu.Unlock()
	}
	return nil
}

// Provider returns one provider for point implemented by extension.
func (b *Broker) Provider(point extensions.PointID, extension extensions.ExtensionID) (any, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	state, ok := b.extensions[extension]
	if !ok {
		return nil, fmt.Errorf("extension %q is not registered", extension)
	}
	for _, provider := range state.extension.Providers {
		if provider.Point == point {
			return provider.Impl, nil
		}
	}
	return nil, fmt.Errorf("extension %q does not provide point %q", extension, point)
}

// Providers returns all providers for point. The order is unspecified and not
// part of the contract.
func (b *Broker) Providers(point extensions.PointID) []extensions.ResolvedProvider {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.providersLocked(point)
}

// providersLocked enumerates every registered provider, including built-ins,
// without acquiring the lock.
func (b *Broker) providersLocked(point extensions.PointID) []extensions.ResolvedProvider {
	var providers []extensions.ResolvedProvider
	for _, id := range b.order {
		state := b.extensions[id]
		for _, provider := range state.extension.Providers {
			if provider.Point == point {
				providers = append(providers, extensions.ResolvedProvider{
					Identity: state.identity,
					Impl:     provider.Impl,
				})
			}
		}
	}
	return providers
}

func (b *Broker) resolveOrder() ([]extensions.ExtensionID, error) {
	dependencies := make(map[extensions.ExtensionID][]extensions.ExtensionID, len(b.extensions))
	for _, id := range b.order {
		state := b.extensions[id]
		for _, dep := range state.extension.Dependencies {
			if dep.Extension != "" {
				if _, ok := b.extensions[dep.Extension]; !ok {
					if dep.Optional {
						continue
					}
					return nil, fmt.Errorf("extension %q requires missing extension %q", id, dep.Extension)
				}
				dependencies[id] = append(dependencies[id], dep.Extension)
				continue
			}

			// A point dependency orders the consumer after every current provider.
			providers := b.providersLocked(dep.Point)
			if len(providers) == 0 {
				if dep.Optional {
					continue
				}
				return nil, fmt.Errorf("extension %q requires missing point %q", id, dep.Point)
			}
			for _, provider := range providers {
				dependencies[id] = append(dependencies[id], provider.Identity.ID)
			}
		}
	}

	var resolved []extensions.ExtensionID
	permanent := make(map[extensions.ExtensionID]struct{}, len(b.extensions))
	temporary := make(map[extensions.ExtensionID]struct{}, len(b.extensions))
	var visit func(extensions.ExtensionID, []extensions.ExtensionID) error
	visit = func(id extensions.ExtensionID, stack []extensions.ExtensionID) error {
		if _, ok := permanent[id]; ok {
			return nil
		}
		if _, ok := temporary[id]; ok {
			cycleStart := slices.Index(stack, id)
			cycle := append(stack[cycleStart:], id)
			return fmt.Errorf("extension dependency cycle: %s", joinExtensionIDs(cycle))
		}
		temporary[id] = struct{}{}
		stack = append(stack, id)
		for _, dep := range dependencies[id] {
			if dep == id {
				continue
			}
			if err := visit(dep, stack); err != nil {
				return err
			}
		}
		delete(temporary, id)
		permanent[id] = struct{}{}
		resolved = append(resolved, id)
		return nil
	}
	for _, id := range b.order {
		if err := visit(id, nil); err != nil {
			return nil, err
		}
	}
	return resolved, nil
}

// Shutdown stops initialized extensions in reverse dependency order. Extensions
// whose Init did not run are skipped.
func (b *Broker) Shutdown(ctx context.Context) error {
	// Snapshot hooks under the lock, then call them without it because Shutdown
	// callbacks may resolve providers.
	type hook struct {
		id extensions.ExtensionID
		fn func(context.Context) error
	}
	var hooks []hook
	b.mu.RLock()
	for _, v := range slices.Backward(b.initOrder) {
		state := b.extensions[v]
		if !state.initialized || state.extension.Shutdown == nil {
			continue
		}
		hooks = append(hooks, hook{state.extension.ID, state.extension.Shutdown})
	}
	b.mu.RUnlock()

	var errs []error
	for _, h := range hooks {
		if err := h.fn(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown extension %q: %w", h.id, err))
		}
	}
	return errors.Join(errs...)
}

func joinExtensionIDs(ids []extensions.ExtensionID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = string(id)
	}
	return strings.Join(parts, " -> ")
}

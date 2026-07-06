// Package host runs a set of extensions for a host process (such as the Moby
// daemon) and resolves their point providers. It is the public entry point to
// the extension runtime: it wraps the in-process broker, which is an
// implementation detail, so a host depends only on this package and the public
// extension contracts.
//
// This is the in-process host: it registers and initializes extensions compiled
// into the host process. Launching out-of-process extension binaries is layered
// on top of this later, without changing the host-facing API.
package host

import (
	"context"

	"github.com/moby/moby/v2/internal/extensions"
	"github.com/moby/moby/v2/internal/extensions/internal/broker"
)

// Options configures a [Host].
type Options struct {
	// Extensions are in-process extensions to register.
	Extensions []extensions.Extension
	// ExtensionConfig is each extension's configuration keyed by extension id. It
	// is delivered to each in-process extension via Init, so an extension is
	// configured by id.
	ExtensionConfig map[extensions.ExtensionID]extensions.Config
}

// Host runs extensions and resolves their point providers. It satisfies
// [extensions.Resolver].
type Host struct {
	broker *broker.Broker
}

// New registers the in-process extensions and initializes them. On any error it
// shuts down whatever it started (via the broker).
//
// A single failing extension -- a conflict, an Init error -- fails the whole
// call, by design: the host loads all-or-nothing rather than silently starting
// with a degraded extension set. The caller decides what that means (the daemon
// treats it as a startup failure).
func New(ctx context.Context, opts Options) (_ *Host, retErr error) {
	b := broker.New()
	// Unwind on any failure: the broker shuts down only what it initialized.
	defer func() {
		if retErr != nil {
			_ = b.Shutdown(context.Background())
		}
	}()
	for _, ext := range opts.Extensions {
		if err := b.Register(ext); err != nil {
			return nil, err
		}
	}
	if err := b.Init(ctx, opts.ExtensionConfig); err != nil {
		return nil, err
	}
	return &Host{broker: b}, nil
}

// Provider returns one provider for point implemented by extension.
func (h *Host) Provider(point extensions.PointID, extension extensions.ExtensionID) (any, error) {
	return h.broker.Provider(point, extension)
}

// SingleProvider returns the only provider for point.
func (h *Host) SingleProvider(point extensions.PointID) (any, error) {
	return h.broker.SingleProvider(point)
}

// Providers returns all providers for point.
func (h *Host) Providers(point extensions.PointID) []extensions.ResolvedProvider {
	return h.broker.Providers(point)
}

// Shutdown stops every running extension in reverse dependency order (via the
// broker).
func (h *Host) Shutdown(ctx context.Context) error {
	return h.broker.Shutdown(ctx)
}

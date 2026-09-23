//go:generate go tool mobyextgen

// Package daemonconfigv0 defines the read-only daemon configuration snapshot
// available to Moby extensions.
package daemonconfigv0

import (
	"context"

	"github.com/moby/extensions"
)

// DaemonConfig provides an immutable snapshot of extension bootstrap settings.
// It does not expose the daemon's full configuration or a generic lookup API.
type DaemonConfig interface {
	// Get returns a fresh copy of the snapshot.
	Get(ctx context.Context, req *GetRequest) (*GetResponse, error)
}

// Point is single-cardinality for the daemon configuration snapshot.
// The daemon registers its builtin provider for internal dependencies and does
// not publish it on the daemon socket.
var Point = extensions.DefineSinglePoint[DaemonConfig]("org.mobyproject.extension.daemon.config.v0")

// GetRequest requests the daemon configuration snapshot.
type GetRequest struct{}

// GetResponse contains the immutable extension bootstrap settings.
type GetResponse struct {
	// RootDir is the absolute path to the daemon's persistent data root,
	// not its execution root.
	RootDir string `pb:"1"`
}

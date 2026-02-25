package identitycache

import (
	"context"
	"time"

	imagetypes "github.com/moby/moby/api/types/image"
)

// Entry contains a persisted image signature cache record.
type Entry struct {
	CachedAt  time.Time
	ExpiresAt time.Time
	Signature *imagetypes.SignatureIdentity
}

// Backend is a persistent storage backend for image signature cache entries.
type Backend interface {
	Load(ctx context.Context, cacheKey string, now time.Time) (Entry, bool, error)
	Store(ctx context.Context, cacheKey string, entry Entry, now time.Time) error
	Close() error
}

type nopBackend struct{}

// NewNopBackend returns a backend that never persists or returns entries.
func NewNopBackend() Backend {
	return nopBackend{}
}

func (nopBackend) Load(context.Context, string, time.Time) (Entry, bool, error) {
	return Entry{}, false, nil
}

func (nopBackend) Store(context.Context, string, Entry, time.Time) error {
	return nil
}

func (nopBackend) Close() error {
	return nil
}

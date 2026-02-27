package identitycache

import (
	"sort"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestBoltBackendWalkIncludesExpiredEntriesUntilPrune(t *testing.T) {
	ctx := t.Context()
	now := time.Now().UTC()

	backend, err := NewBoltDBBackend(t.TempDir())
	assert.NilError(t, err)
	defer backend.Close()

	boltbk := backend.(*boltBackend)

	assert.NilError(t, boltbk.Store(ctx, "fresh", Entry{
		CachedAt:  now.Add(-time.Minute),
		ExpiresAt: now.Add(time.Hour),
	}, now))
	assert.NilError(t, boltbk.Store(ctx, "expired", Entry{
		CachedAt:  now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-time.Minute),
	}, now))

	var keys []string
	assert.NilError(t, boltbk.Walk(ctx, now, func(cacheKey string, _ Entry) error {
		keys = append(keys, cacheKey)
		return nil
	}))
	sort.Strings(keys)
	assert.Check(t, is.DeepEqual(keys, []string{"expired", "fresh"}))

	assert.NilError(t, boltbk.PruneExpired(ctx, now))

	keys = nil
	assert.NilError(t, boltbk.Walk(ctx, now, func(cacheKey string, _ Entry) error {
		keys = append(keys, cacheKey)
		return nil
	}))
	assert.Check(t, is.DeepEqual(keys, []string{"fresh"}))
}

func TestBoltBackendLoadExpiredReturnsMissWithoutDelete(t *testing.T) {
	ctx := t.Context()
	now := time.Now().UTC()

	backend, err := NewBoltDBBackend(t.TempDir())
	assert.NilError(t, err)
	defer backend.Close()

	boltbk := backend.(*boltBackend)

	assert.NilError(t, boltbk.Store(ctx, "expired", Entry{
		CachedAt:  now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-time.Minute),
	}, now))

	_, ok, err := boltbk.Load(ctx, "expired", now)
	assert.NilError(t, err)
	assert.Check(t, !ok)

	seen := false
	assert.NilError(t, boltbk.Walk(ctx, now, func(cacheKey string, _ Entry) error {
		if cacheKey == "expired" {
			seen = true
		}
		return nil
	}))
	assert.Check(t, seen)
}

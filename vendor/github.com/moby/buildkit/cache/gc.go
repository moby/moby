package cache

import (
	"context"
	"errors"
	"time"
)

// GCPolicy defines policy for garbage collection
type GCPolicy struct {
	MaxSize         uint64
	MaxKeepDuration time.Duration
}

// // CachePolicy defines policy for keeping a resource in cache
// type CachePolicy struct {
// 	Priority int
// 	LastUsed time.Time
// }
//
// func defaultCachePolicy() CachePolicy {
// 	return CachePolicy{Priority: 10, LastUsed: time.Now()}
// }

func (cm *cacheManager) GC(ctx context.Context) error {
	return errors.New("GC not implemented")
}

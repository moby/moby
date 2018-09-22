package worker

import (
	"math"

	"github.com/moby/buildkit/client"
)

const defaultCap int64 = 2e9 // 2GB

// tempCachePercent represents the percentage ratio of the cache size in bytes to temporarily keep for a short period of time (couple of days)
// over the total cache size in bytes. Because there is no perfect value, a mathematically pleasing one was chosen.
// The value is approximately 13.8
const tempCachePercent = math.E * math.Pi * math.Phi

// DefaultGCPolicy returns a default builder GC policy
func DefaultGCPolicy(p string, defaultKeepBytes int64) []client.PruneInfo {
	keep := defaultKeepBytes
	if defaultKeepBytes == 0 {
		keep = detectDefaultGCCap(p)
	}

	tempCacheKeepBytes := int64(math.Round(float64(keep) / 100. * float64(tempCachePercent)))
	const minTempCacheKeepBytes = 512 * 1e6 // 512MB
	if tempCacheKeepBytes < minTempCacheKeepBytes {
		tempCacheKeepBytes = minTempCacheKeepBytes
	}

	return []client.PruneInfo{
		// if build cache uses more than 512MB delete the most easily reproducible data after it has not been used for 2 days
		{
			Filter:       []string{"type==source.local,type==exec.cachemount,type==source.git.checkout"},
			KeepDuration: 48 * 3600, // 48h
			KeepBytes:    tempCacheKeepBytes,
		},
		// remove any data not used for 60 days
		{
			KeepDuration: 60 * 24 * 3600, // 60d
			KeepBytes:    keep,
		},
		// keep the unshared build cache under cap
		{
			KeepBytes: keep,
		},
		// if previous policies were insufficient start deleting internal data to keep build cache under cap
		{
			All:       true,
			KeepBytes: keep,
		},
	}
}

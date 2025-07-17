package worker

import (
	"math"
	"time"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/disk"
)

const (
	defaultReservedSpaceBytes      int64 = 2e9 // 2GB
	defaultReservedSpacePercentage int64 = 10
	defaultMaxUsedPercentage       int64 = 80
	defaultMinFreePercentage       int64 = 20
)

// tempCachePercent represents the percentage ratio of the cache size in bytes to temporarily keep for a short period of time (couple of days)
// over the total cache size in bytes. Because there is no perfect value, a mathematically pleasing one was chosen.
// The value is approximately 13.8
const tempCachePercent = math.E * math.Pi * math.Phi

// DefaultGCPolicy returns a default builder GC policy
func DefaultGCPolicy(p string, reservedSpace, maxUsedSpace, minFreeSpace int64) []client.PruneInfo {
	if reservedSpace == 0 && maxUsedSpace == 0 && minFreeSpace == 0 {
		// Only check the disk if we need to fill in an inferred value.
		if dstat, err := disk.GetDiskStat(p); err == nil {
			// Fill in default values only if we can read the disk.
			reservedSpace = diskPercentage(dstat, defaultReservedSpacePercentage)
			maxUsedSpace = diskPercentage(dstat, defaultMaxUsedPercentage)
			minFreeSpace = diskPercentage(dstat, defaultMinFreePercentage)
		} else {
			// Fill in only reserved space if we cannot read the disk.
			reservedSpace = defaultReservedSpaceBytes
		}
	}

	tempCacheReservedSpace := int64(math.Round(float64(reservedSpace) / 100. * float64(tempCachePercent)))
	const minTempCacheReservedSpace = 512 * 1e6 // 512MB
	if tempCacheReservedSpace < minTempCacheReservedSpace {
		tempCacheReservedSpace = minTempCacheReservedSpace
	}

	return []client.PruneInfo{
		// if build cache uses more than 512MB delete the most easily reproducible data after it has not been used for 2 days
		{
			Filter:       []string{"type==source.local,type==exec.cachemount,type==source.git.checkout"},
			KeepDuration: 48 * time.Hour,
			MaxUsedSpace: tempCacheReservedSpace,
		},
		// remove any data not used for 60 days
		{
			KeepDuration:  60 * 24 * time.Hour,
			ReservedSpace: reservedSpace,
			MaxUsedSpace:  maxUsedSpace,
			MinFreeSpace:  minFreeSpace,
		},
		// keep the unshared build cache under cap
		{
			ReservedSpace: reservedSpace,
			MaxUsedSpace:  maxUsedSpace,
			MinFreeSpace:  minFreeSpace,
		},
		// if previous policies were insufficient start deleting internal data to keep build cache under cap
		{
			All:           true,
			ReservedSpace: reservedSpace,
			MaxUsedSpace:  maxUsedSpace,
			MinFreeSpace:  minFreeSpace,
		},
	}
}

func diskPercentage(dstat disk.DiskStat, percentage int64) int64 {
	avail := dstat.Total / percentage
	return (avail/(1<<30) + 1) * 1e9 // round up
}

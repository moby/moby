package config

const defaultCap int64 = 2e9 // 2GB

func DefaultGCPolicy(p string, keep int64) []GCPolicy {
	if keep == 0 {
		keep = DetectDefaultGCCap(p)
	}
	return []GCPolicy{
		// if build cache uses more than 512MB delete the most easily reproducible data after it has not been used for 2 days
		{
			Filters:      []string{"type==source.local,type==exec.cachemount,type==source.git.checkout"},
			KeepDuration: 48 * 3600, // 48h
			KeepBytes:    512 * 1e6, // 512MB
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

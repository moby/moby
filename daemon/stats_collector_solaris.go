// +build solaris

package daemon

// platformNewStatsCollector performs platform specific initialization of the
// statsCollector structure. This is a no-op on Solaris.
func platformNewStatsCollector(s *statsCollector) {
}

// getSystemCPUUsage returns the host system's cpu usage in
// nanoseconds. An error is returned if the format of the underlying
// file does not match. This is a no-op on Solaris.
func (s *statsCollector) getSystemCPUUsage() (uint64, error) {
	return 0, nil
}

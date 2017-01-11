// +build windows

package daemon

// platformNewStatsCollector performs platform specific initialisation of the
// statsCollector structure. This is a no-op on Windows.
func platformNewStatsCollector(s *statsCollector) {
}

// getSystemCPUUsage returns the host system's cpu usage in
// nanoseconds. An error is returned if the format of the underlying
// file does not match. This is a no-op on Windows.
func (s *statsCollector) getSystemCPUUsage() (uint64, error) {
	return 0, nil
}

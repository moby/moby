package stats // import "github.com/docker/docker/daemon/stats"

func (s *Collector) getSystemCPUUsage() (uint64, error) {
	return 0, nil
}

func (s *Collector) getNumberOnlineCPUs() (uint32, error) {
	return 0, nil
}

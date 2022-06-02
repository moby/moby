package portallocator

const (
	// defaultPortRangeStart indicates the first port in port range
	defaultPortRangeStart = 60000
	// defaultPortRangeEnd indicates the last port in port range
	defaultPortRangeEnd = 65000
)

func getDynamicPortRange() (start int, end int, err error) {
	return defaultPortRangeStart, defaultPortRangeEnd, nil
}
